package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/solutions"
)

type solutionDraftRequest struct {
	DemandID         string `json:"demand_id"`
	ExpectedRevision uint   `json:"expected_revision"`
	BaseRevisionID   uint   `json:"base_revision_id"`
	Title            string `json:"title"`
	Markdown         string `json:"markdown"`
}

func (s *Server) handleGetSolutionWorkspace(w http.ResponseWriter, r *http.Request) {
	demandID := strings.TrimSpace(r.URL.Query().Get("demand_id"))
	if demandID == "" {
		http.Error(w, "demand_id is required", http.StatusBadRequest)
		return
	}
	workspace, err := s.solutions.GetWorkspace(r.Context(), demandID)
	if errors.Is(err, solutions.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false, "demand_id": demandID})
		return
	}
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	if workspace.Working == nil && len(workspace.Jobs) == 0 && len(workspace.Sources) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"exists": false, "demand_id": demandID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": true, "workspace": workspace})
}

func (s *Server) handleSaveSolutionDraft(w http.ResponseWriter, r *http.Request) {
	var request solutionDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if !demandExists(request.DemandID) {
		http.Error(w, "demand not found", http.StatusNotFound)
		return
	}
	var workspace solutions.Workspace
	var err error
	if request.ExpectedRevision == 0 {
		workspace, err = s.solutions.EnsureDraft(r.Context(), solutions.EnsureDraftCommand{
			DemandID: request.DemandID, Title: request.Title, Markdown: request.Markdown, Actor: authenticatedActor(r),
		})
	} else {
		workspace, err = s.solutions.SaveDraft(r.Context(), solutions.SaveDraftCommand{
			DemandID: request.DemandID, ExpectedRevision: request.ExpectedRevision,
			BaseRevisionID: request.BaseRevisionID, Title: request.Title,
			Markdown: request.Markdown, Actor: authenticatedActor(r),
		})
	}
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace})
}

func (s *Server) handleForkSolutionDraft(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DemandID         string `json:"demand_id"`
		ExpectedRevision uint   `json:"expected_revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	workspace, err := s.solutions.ForkDraft(r.Context(), solutions.ForkDraftCommand{
		DemandID: request.DemandID, ExpectedRevision: request.ExpectedRevision, Actor: authenticatedActor(r),
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace})
}

func (s *Server) handleRequestSolutionPolish(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DemandID       string `json:"demand_id"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var demand db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", strings.TrimSpace(request.DemandID)).First(&demand).Error; err != nil {
		http.Error(w, "demand not found", http.StatusNotFound)
		return
	}
	if _, err := s.ensureSolutionDraftForDemand(r, demand); err != nil {
		writeSolutionError(w, err)
		return
	}
	job, replayed, err := s.solutions.RequestPolish(r.Context(), solutions.RequestPolishCommand{
		DemandID: demand.TaskID, ProjectKey: demand.ProjectKey, RequestedBy: authenticatedActor(r),
		IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	status := http.StatusAccepted
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"job": job, "replayed": replayed})
}

func (s *Server) handleRetrySolutionJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := pathUint(r, "id")
	if err != nil || jobID == 0 {
		http.Error(w, "invalid solution job id", http.StatusBadRequest)
		return
	}
	var request struct {
		DemandID string `json:"demand_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	job, replayed, err := s.solutions.RetryFailedPolish(r.Context(), solutions.RetryFailedPolishCommand{
		DemandID: request.DemandID, JobID: jobID, RequestedBy: authenticatedActor(r),
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	workspace, err := s.solutions.GetWorkspace(r.Context(), request.DemandID)
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	status := http.StatusAccepted
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"job": job, "replayed": replayed, "workspace": workspace})
}

func (s *Server) handleApplySolutionCandidate(w http.ResponseWriter, r *http.Request) {
	candidateID, err := strconv.ParseUint(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || candidateID == 0 {
		http.Error(w, "invalid candidate id", http.StatusBadRequest)
		return
	}
	var request struct {
		DemandID         string `json:"demand_id"`
		ExpectedRevision uint   `json:"expected_revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	workspace, err := s.solutions.ApplyCandidate(r.Context(), solutions.ApplyCandidateCommand{
		DemandID: request.DemandID, CandidateID: uint(candidateID),
		ExpectedRevision: request.ExpectedRevision, Actor: authenticatedActor(r),
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace})
}

func (s *Server) handlePublishSolution(w http.ResponseWriter, r *http.Request) {
	var request struct {
		DemandID         string `json:"demand_id"`
		ExpectedRevision uint   `json:"expected_revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	link := solutionPublicLinkForRequest(s.config.Server.PublicURL, r, request.DemandID)
	if s.config.Jira.Enabled && !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		http.Error(w, "server.public_url is required before publishing a Jira solution link", http.StatusUnprocessableEntity)
		return
	}
	workspace, err := s.solutions.Publish(r.Context(), solutions.PublishCommand{
		DemandID: request.DemandID, ExpectedRevision: request.ExpectedRevision,
		Actor: authenticatedActor(r), Link: link,
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	if s.solutionCatalog != nil {
		if _, syncErr := s.solutionCatalog.ProcessPending(r.Context(), 1); syncErr != nil {
			// Publication already committed with an idempotent catalog job. The
			// background worker and periodic reconciler will repair this projection.
			log.Printf("Solution catalog immediate sync failed for %s: %v", request.DemandID, syncErr)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace, "link": link})
}

func solutionPublicLink(publicURL, demandID string) string {
	base := strings.TrimSpace(publicURL)
	fallbackLink := fmt.Sprintf("/?tab=schedule&demand=%s&solution=1", url.QueryEscape(strings.TrimSpace(demandID)))
	if base == "" {
		return fallbackLink
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fallbackLink
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/"
	query := parsed.Query()
	query.Set("tab", "schedule")
	query.Set("demand", strings.TrimSpace(demandID))
	query.Set("solution", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func solutionPublicLinkForRequest(publicURL string, request *http.Request, demandID string) string {
	link := solutionPublicLink(publicURL, demandID)
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	return solutionPublicLink(browserRequestOrigin(request), demandID)
}

func browserRequestOrigin(request *http.Request) string {
	if request == nil {
		return ""
	}
	origins := request.Header.Values("Origin")
	if len(origins) != 1 {
		return ""
	}
	origin := strings.TrimSpace(origins[0])
	if origin == "" || strings.ContainsAny(origin, " ,") {
		return ""
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (s *Server) handleListSolutionPrompts(w http.ResponseWriter, r *http.Request) {
	prompts, err := s.solutions.ListPrompts(r.Context())
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": prompts})
}

func (s *Server) handleSaveSolutionPrompt(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Purpose      string `json:"purpose"`
		ScopeType    string `json:"scope_type"`
		ScopeID      string `json:"scope_id"`
		Name         string `json:"name"`
		SystemPrompt string `json:"system_prompt"`
		Activate     bool   `json:"activate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	prompt, err := s.solutions.SavePrompt(r.Context(), solutions.SavePromptCommand{
		Purpose:   request.Purpose,
		ScopeType: request.ScopeType, ScopeID: request.ScopeID, Name: request.Name,
		SystemPrompt: request.SystemPrompt, Activate: request.Activate, Actor: authenticatedActor(r),
	})
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	_ = userdb.RecordAuditLog(db.DB, authenticatedActor(r), "solution_prompt_create", "solution_prompt", strconv.FormatUint(uint64(prompt.ID), 10), fmt.Sprintf("scope=%s:%s version=%d active=%t", prompt.ScopeType, prompt.ScopeID, prompt.Version, prompt.Status == "active"), r.RemoteAddr)
	writeJSON(w, http.StatusCreated, map[string]any{"prompt": prompt})
}

func (s *Server) handleActivateSolutionPrompt(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil || id == 0 {
		http.Error(w, "invalid prompt id", http.StatusBadRequest)
		return
	}
	prompt, err := s.solutions.ActivatePrompt(r.Context(), id, authenticatedActor(r))
	if err != nil {
		writeSolutionError(w, err)
		return
	}
	_ = userdb.RecordAuditLog(db.DB, authenticatedActor(r), "solution_prompt_activate", "solution_prompt", strconv.FormatUint(uint64(prompt.ID), 10), fmt.Sprintf("scope=%s:%s version=%d", prompt.ScopeType, prompt.ScopeID, prompt.Version), r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"prompt": prompt})
}

func (s *Server) handleTestSolutionPrompt(w http.ResponseWriter, r *http.Request) {
	var request struct {
		SystemPrompt string `json:"system_prompt"`
		Markdown     string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(request.SystemPrompt) == "" || strings.TrimSpace(request.Markdown) == "" {
		http.Error(w, "system_prompt and markdown are required", http.StatusBadRequest)
		return
	}
	output, err := queryServerLLMContext(r.Context(), s.config, request.SystemPrompt+solutionSourceSafetyBoundary, "当前 Markdown：\n\n"+request.Markdown)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"markdown": strings.TrimSpace(output)})
}

func (s *Server) ensureSolutionDraftForDemand(r *http.Request, demand db.TaskTelemetry) (solutions.Workspace, error) {
	return s.ensureSolutionDraftForTask(r.Context(), demand, authenticatedActor(r))
}

func (s *Server) ensureSolutionDraftForTask(ctx context.Context, demand db.TaskTelemetry, actor string) (solutions.Workspace, error) {
	markdown := solutionInitialMarkdown(demand)
	return s.solutions.EnsureDraft(ctx, solutions.EnsureDraftCommand{
		DemandID: demand.TaskID, Title: demand.Title, Markdown: markdown, Actor: actor,
	})
}

func solutionInitialMarkdown(demand db.TaskTelemetry) string {
	markdown := fmt.Sprintf("# %s\n\n%s", strings.TrimSpace(demand.Title), strings.TrimSpace(demand.Description))
	if strings.TrimSpace(demand.Description) == "" {
		markdown = fmt.Sprintf("# %s\n\n## 待确认事项\n\n- 需求背景与完整方案待补充", strings.TrimSpace(demand.Title))
	}
	return markdown
}

func demandExists(demandID string) bool {
	var count int64
	return db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", strings.TrimSpace(demandID)).Count(&count).Error == nil && count == 1
}

func (s *Server) withGlobalSuperAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count int64
		err := db.DB.Table("user_group_memberships ugm").
			Joins("join users on users.id = ugm.user_id").
			Joins("join user_groups on user_groups.id = ugm.user_group_id").
			Where("users.username = ? AND user_groups.name = ? AND (ugm.scope = ? OR ugm.scope = '')", r.Header.Get("x-authenticated-user-id"), "super_admin", "global").
			Count(&count).Error
		if err != nil || count == 0 {
			http.Error(w, "global super administrator required", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func writeSolutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, solutions.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, solutions.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, solutions.ErrImmutable):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, solutions.ErrInvalid), errors.Is(err, solutions.ErrTooLarge):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, solutions.ErrIntegrity):
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
