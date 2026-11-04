package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subconv-next/internal/model"
)

func globalRuleFixture(key string, enabled bool) model.CustomRule {
	return model.CustomRule{
		Key:            key,
		Label:          "全局规则 " + key,
		Enabled:        enabled,
		TargetMode:     "new_group",
		SourceType:     "http",
		Behavior:       "domain",
		Format:         "mrs",
		URL:            "https://example.com/rules/" + key + ".mrs",
		Interval:       86400,
		InsertPosition: "before_match",
	}
}

func doGlobalRulesRequest(t *testing.T, server *Server, method, path, body string) (int, globalRuleListResponse) {
	t.Helper()
	var reqBody *strings.Reader
	if body == "" {
		reqBody = strings.NewReader("")
	} else {
		reqBody = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	var parsed globalRuleListResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	return rec.Code, parsed
}

func TestGlobalRulesCRUDAPI(t *testing.T) {
	server, _ := newTestServer(t, model.DefaultConfig())

	ruleJSON, err := json.Marshal(globalRuleRequest{Rule: globalRuleFixture("ai_rules", true)})
	if err != nil {
		t.Fatalf("Marshal rule error = %v", err)
	}

	if code, _ := doGlobalRulesRequest(t, server, http.MethodPost, "/api/global-rules", string(ruleJSON)); code != http.StatusOK {
		t.Fatalf("POST create code = %d", code)
	}
	// Duplicate key must be rejected.
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPost, "/api/global-rules", string(ruleJSON)); code != http.StatusConflict {
		t.Fatalf("POST duplicate code = %d, want 409", code)
	}
	// Invalid key must be rejected.
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPost, "/api/global-rules", `{"rule":{"key":"BAD KEY","label":"x","enabled":true,"source_type":"inline","insert_position":"before_match","payload":["a.com"]}}`); code != http.StatusBadRequest {
		t.Fatalf("POST invalid key code = %d, want 400", code)
	}

	code, listed := doGlobalRulesRequest(t, server, http.MethodGet, "/api/global-rules", "")
	if code != http.StatusOK || len(listed.Rules) != 1 || listed.Rules[0].Key != "ai_rules" {
		t.Fatalf("GET rules = %+v (code %d)", listed, code)
	}

	updated := globalRuleFixture("ai_rules", false)
	updated.Label = "已停用"
	updatedJSON, _ := json.Marshal(globalRuleRequest{Rule: updated})
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPut, "/api/global-rules/ai_rules", string(updatedJSON)); code != http.StatusOK {
		t.Fatalf("PUT update code = %d", code)
	}
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPut, "/api/global-rules/missing_rule", string(updatedJSON)); code != http.StatusNotFound {
		t.Fatalf("PUT missing code = %d, want 404", code)
	}
	_, listed = doGlobalRulesRequest(t, server, http.MethodGet, "/api/global-rules", "")
	if len(listed.Rules) != 1 || listed.Rules[0].Enabled || listed.Rules[0].Label != "已停用" {
		t.Fatalf("after PUT rules = %+v", listed.Rules)
	}

	if code, _ := doGlobalRulesRequest(t, server, http.MethodDelete, "/api/global-rules/ai_rules", ""); code != http.StatusOK {
		t.Fatalf("DELETE code = %d", code)
	}
	if code, _ := doGlobalRulesRequest(t, server, http.MethodDelete, "/api/global-rules/ai_rules", ""); code != http.StatusNotFound {
		t.Fatalf("DELETE twice code = %d, want 404", code)
	}
	_, listed = doGlobalRulesRequest(t, server, http.MethodGet, "/api/global-rules", "")
	if len(listed.Rules) != 0 {
		t.Fatalf("after DELETE rules = %+v", listed.Rules)
	}
}

func TestWithGlobalCustomRulesSharedAcrossWorkspaces(t *testing.T) {
	server, _ := newTestServer(t, model.DefaultConfig())

	ruleJSON, _ := json.Marshal(globalRuleRequest{Rule: globalRuleFixture("shared_mrs", true)})
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPost, "/api/global-rules", string(ruleJSON)); code != http.StatusOK {
		t.Fatalf("POST create code = %d", code)
	}
	disabled := globalRuleFixture("disabled_mrs", false)
	disabledJSON, _ := json.Marshal(globalRuleRequest{Rule: disabled})
	if code, _ := doGlobalRulesRequest(t, server, http.MethodPost, "/api/global-rules", string(disabledJSON)); code != http.StatusOK {
		t.Fatalf("POST create disabled code = %d", code)
	}

	workspaceCfg := model.DefaultConfig()
	workspaceCfg.Render.CustomRules = []model.CustomRule{
		{Key: "workspace_only", Label: "工作区规则", Enabled: true, TargetMode: "direct", SourceType: "inline", InsertPosition: "before_match", Payload: []string{"old.example"}},
		{Key: "shared_mrs", Label: "旧的本地副本", Enabled: true, TargetMode: "direct", SourceType: "inline", InsertPosition: "before_match", Payload: []string{"stale.example"}},
		{Key: "disabled_mrs", Label: "被全局停用的残留", Enabled: true, TargetMode: "direct", SourceType: "inline", InsertPosition: "before_match", Payload: []string{"ghost.example"}},
	}

	merged := server.withGlobalCustomRules(workspaceCfg)

	byKey := map[string]model.CustomRule{}
	for _, rule := range merged.Render.CustomRules {
		if _, dup := byKey[rule.Key]; dup {
			t.Fatalf("duplicate rule key %q after merge", rule.Key)
		}
		byKey[rule.Key] = rule
	}
	if rule, ok := byKey["shared_mrs"]; !ok || rule.SourceType != "http" || rule.Format != "mrs" {
		t.Fatalf("shared_mrs should come from the global store, got %+v", rule)
	}
	if rule, ok := byKey["workspace_only"]; !ok || rule.Label != "工作区规则" {
		t.Fatalf("workspace_only should survive the merge, got %+v", rule)
	}
	if _, ok := byKey["disabled_mrs"]; ok {
		t.Fatal("disabled global rule must hide stale workspace copies")
	}
	enabledFound := false
	for _, key := range merged.Render.EnabledRules {
		if key == "shared_mrs" {
			enabledFound = true
		}
	}
	if !enabledFound {
		t.Fatalf("enabled rules %v missing shared_mrs", merged.Render.EnabledRules)
	}

	// Every workspace sees the same global rules.
	other := server.withGlobalCustomRules(model.DefaultConfig())
	if len(other.Render.CustomRules) != 1 || other.Render.CustomRules[0].Key != "shared_mrs" {
		t.Fatalf("second workspace rules = %+v", other.Render.CustomRules)
	}
}
