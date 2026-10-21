package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/jiraissuesync"
	"well-ambient/internal/openaccess"
)

func (s *Server) jiraIssueSync() *jiraissuesync.Service {
	out := &jiraissuesync.Service{DB: db.DB, Now: time.Now}
	if s.config != nil {
		out.Repos = append([]config.RepoMapping{}, s.config.GitLab.Repos...)
		out.Projects = config.JiraProjectKeys(&s.config.Jira)
	}
	return out
}

func (s *Server) handleJiraIssueSync(w http.ResponseWriter, r *http.Request) {
	svc := s.jiraIssueSync()
	var value any
	var err error
	switch r.Method {
	case http.MethodPut:
		var req db.JiraIssueSyncConfig
		if err = decodeOpenJSON(r, &req); err == nil {
			value, err = svc.Save(r.Context(), req)
		}
	case http.MethodPost:
		var req struct {
			Version int `json:"version"`
		}
		if err = decodeOpenJSON(r, &req); err != nil {
			break
		}
		var setting db.JiraIssueSyncConfig
		setting, err = svc.Settings(r.Context())
		if err != nil {
			break
		}
		if req.Version != setting.Version {
			err = openaccess.NewError("conflict", "configuration changed; reload first")
			break
		}
		var key string
		key, err = openaccess.RandomID("jira_issue_preview")
		if err == nil {
			value, err = svc.Generate(r.Context(), setting, key)
		}
	default:
		var setting db.JiraIssueSyncConfig
		setting, err = svc.Settings(r.Context())
		if err != nil {
			break
		}
		var latest *jiraissuesync.Preview
		latest, err = svc.Latest(r.Context())
		value = map[string]any{"settings": setting, "projects": svc.Projects, "repositories": svc.Repos, "latest": latest, "mode": "preview_only", "timezone": "Asia/Shanghai", "token_configured": s.config != nil && s.config.GitLab.APIToken != ""}
	}
	if err != nil {
		status := http.StatusInternalServerError
		if openaccess.ErrorCode(err) == "invalid_query" {
			status = http.StatusBadRequest
		}
		if openaccess.ErrorCode(err) == "conflict" {
			status = http.StatusConflict
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) startJiraIssueSyncPreviewWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := s.jiraIssueSync().Tick(ctx); err != nil && ctx.Err() == nil {
			log.Print("Jira Issue preview failed; no remote changes were made")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
