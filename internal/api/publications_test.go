package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subconv-next/internal/config"
	"subconv-next/internal/model"
)

func TestPublicationsListAndOpen(t *testing.T) {
	cfg := model.DefaultConfig()
	cfg.Inline = []model.InlineConfig{
		{Name: "manual", Enabled: true, Content: "ss://YWVzLTI1Ni1nY206cGFzczdAZXhhbXBsZS5jb206NDQz#node"},
	}
	server, cfg := newTestServer(t, cfg)
	workspaceID := createWorkspaceForTest(t, server, cfg)

	refreshReq := httptest.NewRequest(http.MethodPost, withWorkspace("/api/refresh", workspaceID), nil)
	refreshRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status code = %d; body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	refreshBody := decodeRefreshResponse(t, refreshRec)

	ref, err := server.loadWorkspace(workspaceID)
	if err != nil {
		t.Fatalf("loadWorkspace() error = %v", err)
	}
	workspaceCfg := model.DefaultConfig()
	workspaceCfg.Subscriptions = []model.SubscriptionConfig{
		{ID: "source-a", Name: "A", Enabled: true, URL: "https://sub.example.com/api/v1/client?token=1", UserAgent: model.DefaultUserAgent},
		{ID: "source-b", Name: "B", Enabled: true, URL: "https://sub.example.com/other", UserAgent: model.DefaultUserAgent},
	}
	if err := config.WriteJSON(ref.ConfigPath, workspaceCfg); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}

	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/publications", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status code = %d; body=%s", listRec.Code, listRec.Body.String())
	}
	var listed publicationsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if !listed.OK || len(listed.Publications) != 1 {
		t.Fatalf("list response = %+v, want one publication", listed)
	}
	item := listed.Publications[0]
	if item.PublishID != refreshBody.PublishID {
		t.Fatalf("publish_id = %q, want %q", item.PublishID, refreshBody.PublishID)
	}
	if item.WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", item.WorkspaceID, workspaceID)
	}
	if !item.Editable {
		t.Fatal("editable = false, want true for live workspace")
	}
	if item.URL == "" || item.TokenHint == "" {
		t.Fatalf("url/token_hint missing: %+v", item)
	}
	if len(item.SourceHosts) != 1 || item.SourceHosts[0] != "sub.example.com" {
		t.Fatalf("source_hosts = %v, want [sub.example.com]", item.SourceHosts)
	}

	openRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(openRec, httptest.NewRequest(http.MethodPost, "/api/publications/"+refreshBody.PublishID+"/open", nil))
	if openRec.Code != http.StatusOK {
		t.Fatalf("open status code = %d; body=%s", openRec.Code, openRec.Body.String())
	}
	var opened openPublicationResponse
	if err := json.Unmarshal(openRec.Body.Bytes(), &opened); err != nil {
		t.Fatalf("decode open response: %v", err)
	}
	if opened.WorkspaceID != workspaceID {
		t.Fatalf("open workspace_id = %q, want %q", opened.WorkspaceID, workspaceID)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/publications/"+refreshBody.PublishID, strings.NewReader(`{"note":"  客户A 香港  "}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("note update status = %d, want 200; body=%s", patchRec.Code, patchRec.Body.String())
	}
	var patched map[string]any
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("decode note response: %v", err)
	}
	if patched["note"] != "客户A 香港" {
		t.Fatalf("note = %v, want trimmed value", patched["note"])
	}

	longReq := httptest.NewRequest(http.MethodPatch, "/api/publications/"+refreshBody.PublishID, strings.NewReader(`{"note":"`+strings.Repeat("字", maxPublicationNoteRunes+1)+`"}`))
	longReq.Header.Set("Content-Type", "application/json")
	longRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(longRec, longReq)
	if longRec.Code != http.StatusBadRequest {
		t.Fatalf("long note status = %d, want 400; body=%s", longRec.Code, longRec.Body.String())
	}

	notedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(notedRec, httptest.NewRequest(http.MethodGet, "/api/publications", nil))
	var noted publicationsResponse
	if err := json.Unmarshal(notedRec.Body.Bytes(), &noted); err != nil {
		t.Fatalf("decode noted list: %v", err)
	}
	if len(noted.Publications) != 1 || noted.Publications[0].Note != "客户A 香港" {
		t.Fatalf("publications after note = %+v, want note persisted", noted.Publications)
	}

	if err := server.deleteWorkspace(workspaceID); err != nil {
		t.Fatalf("deleteWorkspace() error = %v", err)
	}
	goneRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(goneRec, httptest.NewRequest(http.MethodPost, "/api/publications/"+refreshBody.PublishID+"/open", nil))
	if goneRec.Code != http.StatusGone {
		t.Fatalf("open after workspace removal status = %d, want 410; body=%s", goneRec.Code, goneRec.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, httptest.NewRequest(http.MethodDelete, "/api/publications/"+refreshBody.PublishID, nil))
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", deleteRec.Code, deleteRec.Body.String())
	}
	afterDelete := httptest.NewRecorder()
	server.Handler().ServeHTTP(afterDelete, httptest.NewRequest(http.MethodGet, "/api/publications", nil))
	var remaining publicationsResponse
	if err := json.Unmarshal(afterDelete.Body.Bytes(), &remaining); err != nil {
		t.Fatalf("decode list after delete: %v", err)
	}
	if len(remaining.Publications) != 0 {
		t.Fatalf("publications after delete = %+v, want empty", remaining.Publications)
	}
}

func TestPublicationsHiddenInPublicConverterMode(t *testing.T) {
	cfg := model.DefaultConfig()
	cfg.Service.PublicConverter = true
	server, _ := newTestServer(t, cfg)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/publications", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("public converter list status = %d, want 404", rec.Code)
	}
}
