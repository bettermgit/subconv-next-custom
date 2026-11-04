package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"subconv-next/internal/config"
	"subconv-next/internal/model"
)

// publicationItem is the management view of a published subscription. It is
// only exposed to the authenticated management UI (never in public converter
// mode) because the returned URL contains the full subscription token.
type publicationItem struct {
	PublishID      string   `json:"publish_id"`
	URL            string   `json:"url"`
	TokenHint      string   `json:"token_hint,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
	LastAccessAt   string   `json:"last_access_at,omitempty"`
	AccessCount    int      `json:"access_count"`
	OutputFilename string   `json:"output_filename,omitempty"`
	Note           string   `json:"note,omitempty"`
	SourceHosts    []string `json:"source_hosts"`
	WorkspaceID    string   `json:"workspace_id,omitempty"`
	Editable       bool     `json:"editable"`
}

type publicationsResponse struct {
	OK           bool              `json:"ok"`
	Publications []publicationItem `json:"publications"`
}

type openPublicationResponse struct {
	OK          bool   `json:"ok"`
	PublishID   string `json:"publish_id"`
	WorkspaceID string `json:"workspace_id"`
	URL         string `json:"url,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

func (s *Server) handlePublications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if s.snapshotConfig().Service.PublicConverter {
		http.NotFound(w, r)
		return
	}
	items, err := s.listPublications(r)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "PUBLICATIONS_LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, publicationsResponse{OK: true, Publications: items})
}

func (s *Server) listPublications(r *http.Request) ([]publicationItem, error) {
	items := []publicationItem{}
	entries, err := os.ReadDir(s.publishedRootDir())
	if err != nil {
		if os.IsNotExist(err) {
			return items, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		published, err := s.loadPublishedByID(entry.Name())
		if err != nil || published.Meta.Revoked || strings.TrimSpace(published.Meta.Token) == "" {
			continue
		}
		item := publicationItem{
			PublishID:      published.ID,
			URL:            publishedURL(s.publicOrigin(r), published.Meta.Token, published.Meta.OutputFilename),
			TokenHint:      published.Meta.TokenHint,
			CreatedAt:      formatTime(published.Meta.CreatedAt),
			UpdatedAt:      formatTime(published.Meta.UpdatedAt),
			LastAccessAt:   formatTime(published.Meta.LastAccessAt),
			AccessCount:    published.Meta.AccessCount,
			OutputFilename: published.Meta.OutputFilename,
			Note:           published.Meta.Note,
			SourceHosts:    []string{},
		}
		if hash := strings.TrimSpace(published.Meta.WorkspaceHash); validWorkspaceHash(hash) {
			if ref, err := s.loadWorkspaceByHash(hash); err == nil && strings.TrimSpace(ref.ID) != "" {
				item.WorkspaceID = ref.ID
				// A workspace pinned by a live publication is kept by cleanup,
				// so it remains editable even after the regular TTL elapsed.
				item.Editable = !s.workspaceExpired(ref.Meta) || s.workspaceHasPublishedRefreshBinding(ref)
				if cfg, err := config.Load(ref.ConfigPath); err == nil {
					item.SourceHosts = publicationSourceHosts(cfg)
				}
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].PublishID > items[j].PublishID
	})
	return items, nil
}

func publicationSourceHosts(cfg model.Config) []string {
	hosts := []string{}
	seen := map[string]struct{}{}
	for _, subscription := range cfg.Subscriptions {
		if !subscription.Enabled {
			continue
		}
		parsed, err := url.Parse(strings.TrimSpace(subscription.URL))
		if err != nil {
			continue
		}
		host := strings.TrimSpace(parsed.Hostname())
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
		if len(hosts) >= 5 {
			break
		}
	}
	return hosts
}

func (s *Server) handlePublicationSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/publications/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	publishID := strings.TrimSpace(parts[0])
	if len(parts) == 2 && parts[1] == "open" {
		s.handleOpenPublication(w, r, publishID)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodDelete:
			s.handleDeletePublication(w, r, publishID)
			return
		case http.MethodPatch, http.MethodPut:
			s.handleUpdatePublication(w, r, publishID)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) handleDeletePublication(w http.ResponseWriter, r *http.Request, publishID string) {
	if s.snapshotConfig().Service.PublicConverter {
		http.NotFound(w, r)
		return
	}
	if err := s.deletePublishedForWorkspace(publishID, ""); err != nil {
		if errors.Is(err, errPublishedInvalidID) || errors.Is(err, errWorkspaceNotFound) || os.IsNotExist(err) {
			writeAPIError(w, http.StatusNotFound, "PUBLISHED_NOT_FOUND", "published subscription not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "PUBLISHED_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

const maxPublicationNoteRunes = 120

type updatePublicationRequest struct {
	Note *string `json:"note"`
}

// handleUpdatePublication stores an operator-only note for a published
// subscription. The note lives in the publication meta and is only returned to
// the authenticated management UI; it never reaches the generated YAML.
func (s *Server) handleUpdatePublication(w http.ResponseWriter, r *http.Request, publishID string) {
	if s.snapshotConfig().Service.PublicConverter {
		http.NotFound(w, r)
		return
	}
	var req updatePublicationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Note == nil {
		writeAPIError(w, http.StatusBadRequest, "PUBLICATION_NOTE_REQUIRED", "note is required")
		return
	}
	note := strings.TrimSpace(*req.Note)
	if utf8.RuneCountInString(note) > maxPublicationNoteRunes {
		writeAPIError(w, http.StatusBadRequest, "PUBLICATION_NOTE_TOO_LONG", fmt.Sprintf("note must be at most %d characters", maxPublicationNoteRunes))
		return
	}
	updated, err := s.updatePublishedMeta(publishID, func(ref *publishedRef) error {
		ref.Meta.Note = note
		return nil
	})
	if err != nil {
		if errors.Is(err, errPublishedInvalidID) || errors.Is(err, errWorkspaceNotFound) || os.IsNotExist(err) {
			writeAPIError(w, http.StatusNotFound, "PUBLISHED_NOT_FOUND", "published subscription not found")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "PUBLICATION_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": updated.Meta.Note})
}

func (s *Server) handleOpenPublication(w http.ResponseWriter, r *http.Request, publishID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if s.snapshotConfig().Service.PublicConverter {
		http.NotFound(w, r)
		return
	}
	published, err := s.loadPublishedByID(publishID)
	if err != nil || !publishedRestorable(published) {
		writeAPIError(w, http.StatusNotFound, "PUBLISHED_NOT_FOUND", "published subscription not found")
		return
	}
	hash := strings.TrimSpace(published.Meta.WorkspaceHash)
	if !validWorkspaceHash(hash) {
		writeAPIError(w, http.StatusGone, "PUBLICATION_WORKSPACE_MISSING", "该订阅链接仍可下载，但原始配置已清理，无法继续编辑")
		return
	}
	ref, err := s.loadWorkspaceByHash(hash)
	if err != nil || strings.TrimSpace(ref.ID) == "" {
		writeAPIError(w, http.StatusGone, "PUBLICATION_WORKSPACE_MISSING", "该订阅链接仍可下载，但原始配置已清理，无法继续编辑")
		return
	}
	if existing := strings.TrimSpace(ref.Meta.PublishID); existing != "" && existing != published.ID {
		writeAPIError(w, http.StatusConflict, "PUBLICATION_WORKSPACE_CONFLICT", "该配置已绑定到另一个订阅发布")
		return
	}
	if strings.TrimSpace(ref.Meta.PublishID) != published.ID {
		ref.Meta.PublishID = published.ID
		ref.Meta.LegacyPublishedToken = ""
		ref.Meta.LegacyPublishedAt = time.Time{}
		if err := s.saveWorkspaceMeta(ref); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "WORKSPACE_BIND_FAILED", err.Error())
			return
		}
	}
	ref, err = s.loadWorkspace(ref.ID)
	if err != nil {
		handleWorkspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, openPublicationResponse{
		OK:          true,
		PublishID:   published.ID,
		WorkspaceID: ref.ID,
		URL:         publishedURL(s.publicOrigin(r), published.Meta.Token, published.Meta.OutputFilename),
		ExpiresAt:   formatTime(s.workspaceExpiresAt(ref.Meta)),
	})
}
