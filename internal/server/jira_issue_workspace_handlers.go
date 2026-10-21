package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"well-ambient/internal/db"
	"well-ambient/internal/jiraissuesync"
	"well-ambient/internal/openaccess"
)

func (s *Server) jiraIssueWorkspace() *jiraissuesync.Workspace {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("WELL_JIRA_ISSUE_SYNC_KEY")))
	service := &jiraissuesync.Workspace{DB: db.DB, Key: key}
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()
	if s.config != nil && s.config.AI.Enabled && s.config.AI.APIToken != "" && s.config.AI.BaseURL != "" {
		snapshot := *s.config
		service.AI = func(ctx context.Context, system, user string) (string, error) {
			return queryServerLLMContext(ctx, &snapshot, system, user)
		}
	}
	if s.jiraIssueAnalyze != nil {
		service.AI = s.jiraIssueAnalyze
	}
	return service
}
func (s *Server) handleJiraIssueWorkspace(w http.ResponseWriter, r *http.Request) {
	service := s.jiraIssueWorkspace()
	var value any
	var err error
	status := http.StatusOK
	switch r.Method {
	case http.MethodPut:
		var c jiraissuesync.WorkspaceConfig
		if err = decodeOpenJSON(r, &c); err == nil {
			value, err = service.Save(r.Context(), c)
		}
	case http.MethodPost:
		if r.PathValue("action") == "cancel" {
			var req struct {
				RunID uint `json:"run_id"`
			}
			if err = decodeOpenJSON(r, &req); err == nil {
				err = service.Cancel(r.Context(), req.RunID)
				value = map[string]bool{"cancel_requested": err == nil}
			}
			break
		}
		var req struct {
			Version  int      `json:"version"`
			RepoIDs  []string `json:"repo_ids"`
			Projects []string `json:"projects"`
		}
		if err = decodeOpenJSON(r, &req); err != nil {
			break
		}
		var run db.JiraIssueRun
		var lease string
		run, lease, err = service.Begin(r.Context(), req.Version, r.PathValue("action"))
		if err != nil {
			break
		}
		ctx := s.streamingCtx
		if ctx == nil {
			ctx = context.Background()
		}
		go service.Execute(ctx, run, lease, req.RepoIDs, req.Projects)
		value = run
		status = http.StatusAccepted
	default:
		if raw := r.URL.Query().Get("run_id"); raw != "" {
			id, e := strconv.ParseUint(raw, 10, 32)
			offset, e2 := strconv.Atoi(r.URL.Query().Get("offset"))
			if r.URL.Query().Get("offset") == "" {
				offset = 0
				e2 = nil
			}
			if e != nil || e2 != nil {
				err = openaccess.NewError("invalid_request", "分页参数无效")
			} else {
				value, err = service.RunDetail(r.Context(), uint(id), offset)
			}
		} else {
			value, err = service.View(r.Context())
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status = http.StatusInternalServerError
		message := "同步工作台请求失败"
		switch openaccess.ErrorCode(err) {
		case "invalid_request":
			status = http.StatusBadRequest
			message = err.Error()
		case "conflict":
			status = http.StatusConflict
			message = err.Error()
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
