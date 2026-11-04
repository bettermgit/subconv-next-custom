package api

// Global custom rules are a server-wide rule library shared by every
// workspace. They live outside workspace configs in
// <data>/global_custom_rules.json so saving or deleting a rule once applies
// to all workspaces. Enabled entries are merged into the render config at
// refresh time (workspace entries with the same key are overridden by the
// global store).

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"subconv-next/internal/config"
	"subconv-next/internal/model"
	"subconv-next/internal/storage"
)

const globalCustomRulesFileName = "global_custom_rules.json"

var globalCustomRuleKeyPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

var (
	globalRulesMu sync.Mutex
	errRuleExists = errors.New("custom rule key already exists")
	errRuleAbsent = errors.New("custom rule not found")
)

func (s *Server) globalCustomRulesPath() string {
	return filepath.Join(s.baseDataDir(), globalCustomRulesFileName)
}

func (s *Server) loadGlobalCustomRules() ([]model.CustomRule, error) {
	path := s.globalCustomRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.CustomRule{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []model.CustomRule{}, nil
	}
	var wrapper struct {
		Rules []model.CustomRule `json:"rules"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse %s: %w", globalCustomRulesFileName, err)
	}
	if wrapper.Rules == nil {
		wrapper.Rules = []model.CustomRule{}
	}
	return wrapper.Rules, nil
}

func (s *Server) saveGlobalCustomRules(rules []model.CustomRule) error {
	if err := validateGlobalCustomRules(rules); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Rules []model.CustomRule `json:"rules"`
	}{Rules: rules}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return storage.AtomicWriteFile(s.globalCustomRulesPath(), data, 0o600)
}

func validateGlobalCustomRules(rules []model.CustomRule) error {
	if len(rules) > 64 {
		return fmt.Errorf("global custom rule library is limited to 64 rules")
	}
	cfg := model.DefaultConfig()
	cfg.Render.CustomRules = rules
	return config.Validate(cfg)
}

// withGlobalCustomRules returns a copy of cfg whose render rules contain the
// enabled entries of the global rule library. Global entries win on key
// collisions; disabled global entries hide stale workspace copies.
func (s *Server) withGlobalCustomRules(cfg model.Config) model.Config {
	rules, err := s.loadGlobalCustomRules()
	if err != nil {
		s.appendLog("global custom rules load failed: " + err.Error())
		return cfg
	}
	if len(rules) == 0 {
		return cfg
	}
	globalByKey := make(map[string]model.CustomRule, len(rules))
	globalKeys := make([]string, 0, len(rules))
	for _, rule := range rules {
		key := strings.ToLower(strings.TrimSpace(rule.Key))
		if key == "" {
			continue
		}
		globalByKey[key] = rule
		globalKeys = append(globalKeys, key)
	}

	merged := make([]model.CustomRule, 0, len(cfg.Render.CustomRules)+len(rules))
	for _, rule := range cfg.Render.CustomRules {
		key := strings.ToLower(strings.TrimSpace(rule.Key))
		if _, overridden := globalByKey[key]; overridden {
			continue
		}
		merged = append(merged, rule)
	}
	sort.Strings(globalKeys)
	for _, key := range globalKeys {
		rule := globalByKey[key]
		if !rule.Enabled {
			continue
		}
		merged = append(merged, rule)
	}
	cfg.Render.CustomRules = merged

	enabledSeen := make(map[string]struct{}, len(cfg.Render.EnabledRules))
	for _, key := range cfg.Render.EnabledRules {
		enabledSeen[strings.TrimSpace(key)] = struct{}{}
	}
	for _, key := range globalKeys {
		rule := globalByKey[key]
		if !rule.Enabled {
			continue
		}
		if _, ok := enabledSeen[key]; !ok {
			cfg.Render.EnabledRules = append(cfg.Render.EnabledRules, key)
			enabledSeen[key] = struct{}{}
		}
	}
	return cfg
}

type globalRuleRequest struct {
	Rule model.CustomRule `json:"rule"`
}

type globalRuleListResponse struct {
	OK    bool               `json:"ok"`
	Rules []model.CustomRule `json:"rules"`
}

func (s *Server) handleGlobalRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		globalRulesMu.Lock()
		defer globalRulesMu.Unlock()
		rules, err := s.loadGlobalCustomRules()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "GLOBAL_RULES_READ_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, globalRuleListResponse{OK: true, Rules: rules})
	case http.MethodPost:
		var req globalRuleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		globalRulesMu.Lock()
		defer globalRulesMu.Unlock()
		rules, err := s.loadGlobalCustomRules()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "GLOBAL_RULES_READ_FAILED", err.Error())
			return
		}
		key := strings.ToLower(strings.TrimSpace(req.Rule.Key))
		if !globalCustomRuleKeyPattern.MatchString(key) {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "rule key must match [a-z0-9_-]{1,64}")
			return
		}
		for _, existing := range rules {
			if strings.EqualFold(existing.Key, key) {
				writeAPIError(w, http.StatusConflict, "GLOBAL_RULE_EXISTS", "rule key already exists: "+key)
				return
			}
		}
		req.Rule.Key = key
		rules = append(rules, req.Rule)
		if err := s.saveGlobalCustomRules(rules); err != nil {
			writeAPIError(w, http.StatusBadRequest, "GLOBAL_RULE_INVALID", err.Error())
			return
		}
		s.appendLog("global custom rule added: " + key)
		writeJSON(w, http.StatusOK, globalRuleListResponse{OK: true, Rules: rules})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (s *Server) handleGlobalRuleSubroutes(w http.ResponseWriter, r *http.Request) {
	key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/global-rules/")))
	if key == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "rule key is required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req globalRuleRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		globalRulesMu.Lock()
		defer globalRulesMu.Unlock()
		rules, err := s.loadGlobalCustomRules()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "GLOBAL_RULES_READ_FAILED", err.Error())
			return
		}
		index := -1
		for i, rule := range rules {
			if strings.EqualFold(rule.Key, key) {
				index = i
				break
			}
		}
		if index < 0 {
			writeAPIError(w, http.StatusNotFound, "GLOBAL_RULE_NOT_FOUND", "rule key not found: "+key)
			return
		}
		req.Rule.Key = key
		rules[index] = req.Rule
		if err := s.saveGlobalCustomRules(rules); err != nil {
			writeAPIError(w, http.StatusBadRequest, "GLOBAL_RULE_INVALID", err.Error())
			return
		}
		s.appendLog("global custom rule updated: " + key)
		writeJSON(w, http.StatusOK, globalRuleListResponse{OK: true, Rules: rules})
	case http.MethodDelete:
		globalRulesMu.Lock()
		defer globalRulesMu.Unlock()
		rules, err := s.loadGlobalCustomRules()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "GLOBAL_RULES_READ_FAILED", err.Error())
			return
		}
		next := make([]model.CustomRule, 0, len(rules))
		removed := false
		for _, rule := range rules {
			if strings.EqualFold(rule.Key, key) {
				removed = true
				continue
			}
			next = append(next, rule)
		}
		if !removed {
			writeAPIError(w, http.StatusNotFound, "GLOBAL_RULE_NOT_FOUND", "rule key not found: "+key)
			return
		}
		if err := s.saveGlobalCustomRules(next); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "GLOBAL_RULES_WRITE_FAILED", err.Error())
			return
		}
		s.appendLog("global custom rule deleted: " + key)
		writeJSON(w, http.StatusOK, globalRuleListResponse{OK: true, Rules: next})
	default:
		methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
	}
}
