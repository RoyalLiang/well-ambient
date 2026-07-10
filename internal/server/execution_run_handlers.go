package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

type createExecutionRunRequest struct {
	ExecutionPreflight executionPreflightRequest `json:"preflight"`
	BaseBranch         string                    `json:"base_branch"`
	Title              string                    `json:"title"`
}

type executionRunDTO struct {
	db.ExecutionRun
	Actions []db.ExecutionAction `json:"actions"`
}

type verifyExecutionRunRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func (s *Server) handleListExecutionRuns(w http.ResponseWriter, r *http.Request) {
	demandID := strings.TrimSpace(r.URL.Query().Get("demand_id"))
	query := db.DB.Order("created_at desc")
	if demandID != "" {
		query = query.Where("demand_id = ?", demandID)
	}
	var runs []db.ExecutionRun
	if err := query.Find(&runs).Error; err != nil {
		http.Error(w, "failed to list execution runs", http.StatusInternalServerError)
		return
	}
	items := make([]executionRunDTO, 0, len(runs))
	for _, run := range runs {
		items = append(items, executionRunWithActions(run))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *Server) handleCreateExecutionRun(w http.ResponseWriter, r *http.Request) {
	var req createExecutionRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ExecutionPreflight.Author) == "" {
		req.ExecutionPreflight.Author = authenticatedActor(r)
	}
	preflight := s.buildExecutionPreflight(req.ExecutionPreflight)
	if !preflight.Ready || preflight.Spec == nil || preflight.ReviewContract == nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{"error": "preflight_blocked", "preflight": preflight})
		return
	}
	repo := s.configuredRepo(req.ExecutionPreflight.Repo)
	if repo == nil {
		http.Error(w, "configured repository not found", http.StatusUnprocessableEntity)
		return
	}
	projectRef := firstNonBlank(repo.ProjectID, repo.Path)
	runKey := executionRunKey(req.ExecutionPreflight)
	var existing db.ExecutionRun
	if err := db.DB.Where("run_key = ?", runKey).First(&existing).Error; err == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"reused": true, "run": executionRunWithActions(existing), "preflight": preflight})
		return
	}
	now := time.Now()
	topicBranch := buildTopicBranch(preflight.Spec.DemandID, preflight.Spec.Version, runKey)
	run := db.ExecutionRun{
		RunKey: runKey, DemandID: preflight.Spec.DemandID, DemandSpecVersionID: preflight.Spec.ID,
		ReviewContractID: preflight.ReviewContract.ID, Repo: req.ExecutionPreflight.Repo,
		ProjectRef: projectRef, Provider: "gitlab", BaseBranch: strings.TrimSpace(req.BaseBranch),
		TopicBranch: topicBranch, Status: delivery.RunPreflightReady,
		InitiatedBy: authenticatedActor(r), ServiceIdentity: "well-ambient-ai",
		ChangeSetJSON: encodeJSON(req.ExecutionPreflight.ChangeSet), ChangedPathsJSON: encodeJSON(preflight.ChangedPaths),
		TestCommandsJSON:      encodeJSON(normalizeDeliveryStrings(req.ExecutionPreflight.TestCommands)),
		ResolvedReviewersJSON: encodeJSON(preflight.ReviewerResolution.Reviewers),
		AcceptanceState:       "pending", PipelineStatus: "not_started", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.DB.Create(&run).Error; err != nil {
		http.Error(w, "failed to create execution run", http.StatusInternalServerError)
		return
	}
	_ = recordExecutionAction(run, "preflight", authenticatedActor(r), preflight, map[string]interface{}{"ready": true}, nil)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"reused": false, "run": executionRunWithActions(run), "preflight": preflight})
}

func (s *Server) handleStartExecutionRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid execution run id", http.StatusBadRequest)
		return
	}
	var run db.ExecutionRun
	if err := db.DB.First(&run, id).Error; err != nil {
		http.Error(w, "execution run not found", http.StatusNotFound)
		return
	}
	if run.Status == delivery.RunReviewPending || run.Status == delivery.RunDraftMRCreated || run.Status == delivery.RunAcceptancePending || run.Status == delivery.RunDelivered {
		writeJSON(w, http.StatusOK, map[string]interface{}{"reused": true, "run": executionRunWithActions(run)})
		return
	}
	if run.Status != delivery.RunPreflightReady {
		http.Error(w, "execution run is not ready to start", http.StatusConflict)
		return
	}
	client, err := newGitLabDeliveryClient(&s.config.GitLab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	now := time.Now()
	run.Status = delivery.RunExecuting
	run.StartedAt = &now
	run.UpdatedAt = now
	if err := db.DB.Save(&run).Error; err != nil {
		http.Error(w, "failed to start execution run", http.StatusInternalServerError)
		return
	}

	project, err := client.GetProject(r.Context(), run.ProjectRef)
	if err != nil {
		s.failExecutionRun(&run, "get_project", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	_ = recordExecutionAction(run, "get_project", run.ServiceIdentity, map[string]string{"project_ref": run.ProjectRef}, project, nil)
	baseBranch := firstNonBlank(run.BaseBranch, project.DefaultBranch)
	if strings.EqualFold(baseBranch, run.TopicBranch) || strings.TrimSpace(baseBranch) == "" {
		err = fmt.Errorf("base branch must be a non-empty branch different from the topic branch")
		s.failExecutionRun(&run, "validate_branch", err)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	run.BaseBranch = baseBranch
	_ = db.DB.Save(&run).Error

	reviewers := decodeStringList(run.ResolvedReviewersJSON)
	reviewerIDs, err := client.ResolveReviewerIDs(r.Context(), reviewers)
	if err != nil {
		s.failExecutionRun(&run, "resolve_reviewers", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	_ = recordExecutionAction(run, "resolve_reviewers", run.ServiceIdentity, reviewers, reviewerIDs, nil)

	if err := client.CreateBranch(r.Context(), run.ProjectRef, run.TopicBranch, baseBranch); err != nil {
		s.failExecutionRun(&run, "create_branch", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	_ = recordExecutionAction(run, "create_branch", run.ServiceIdentity, map[string]string{"branch": run.TopicBranch, "ref": baseBranch}, map[string]bool{"created": true}, nil)

	changes := []delivery.FileAction{}
	if err := json.Unmarshal([]byte(run.ChangeSetJSON), &changes); err != nil {
		s.failExecutionRun(&run, "decode_change_set", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	commitMessage := fmt.Sprintf("feat(%s): apply approved autonomous delivery", run.DemandID)
	commit, err := client.Commit(r.Context(), run.ProjectRef, run.TopicBranch, commitMessage, changes)
	if err != nil {
		s.failExecutionRun(&run, "create_commit", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	run.CommitSHA = commit.ID
	_ = db.DB.Save(&run).Error
	_ = recordExecutionAction(run, "create_commit", run.ServiceIdentity, map[string]interface{}{"message": commitMessage, "change_count": len(changes)}, commit, nil)

	var spec db.DemandSpecVersion
	_ = db.DB.First(&spec, run.DemandSpecVersionID).Error
	title := firstNonBlank(spec.Summary, spec.UserGoal, run.DemandID+" autonomous delivery")
	description := buildDraftMRDescription(run, spec)
	mr, err := client.CreateDraftMR(r.Context(), run.ProjectRef, run.TopicBranch, baseBranch, title, description, reviewerIDs)
	if err != nil {
		s.failExecutionRun(&run, "create_draft_mr", err)
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{"error": err.Error(), "run": executionRunWithActions(run)})
		return
	}
	_ = recordExecutionAction(run, "create_draft_mr", run.ServiceIdentity, map[string]interface{}{"source": run.TopicBranch, "target": baseBranch, "reviewer_ids": reviewerIDs}, mr, nil)

	run.MRIID = mr.IID
	run.MRURL = mr.WebURL
	run.Status = delivery.RunReviewPending
	run.PipelineStatus = "pending"
	run.UpdatedAt = time.Now()
	if err := db.DB.Save(&run).Error; err != nil {
		http.Error(w, "Draft MR created but run state update failed", http.StatusInternalServerError)
		return
	}
	if err := s.recordExecutionEvidence(run, commit); err != nil {
		run.BlockReason = "Draft MR created but evidence projection failed: " + err.Error()
		_ = db.DB.Save(&run).Error
		_ = recordExecutionAction(run, "project_evidence", run.ServiceIdentity, nil, nil, err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": run.BlockReason, "run": executionRunWithActions(run)})
		return
	}
	_ = recordExecutionAction(run, "project_evidence", run.ServiceIdentity, nil, map[string]bool{"recorded": true}, nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{"reused": false, "run": executionRunWithActions(run)})
}

func (s *Server) handleCancelExecutionRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid execution run id", http.StatusBadRequest)
		return
	}
	var run db.ExecutionRun
	if err := db.DB.First(&run, id).Error; err != nil {
		http.Error(w, "execution run not found", http.StatusNotFound)
		return
	}
	if run.Status == delivery.RunDelivered {
		http.Error(w, "delivered execution runs cannot be cancelled", http.StatusConflict)
		return
	}
	if delivery.IsTerminalRun(run.Status) {
		writeJSON(w, http.StatusOK, map[string]interface{}{"reused": true, "run": executionRunWithActions(run)})
		return
	}
	now := time.Now()
	run.Status = delivery.RunCancelled
	run.CompletedAt = &now
	run.BlockReason = "cancelled by " + authenticatedActor(r)
	run.UpdatedAt = now
	if err := db.DB.Save(&run).Error; err != nil {
		http.Error(w, "failed to cancel execution run", http.StatusInternalServerError)
		return
	}
	_ = recordExecutionAction(run, "cancel", authenticatedActor(r), nil, map[string]string{"status": run.Status}, nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{"reused": false, "run": executionRunWithActions(run)})
}

func (s *Server) handleRefreshExecutionRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid execution run id", http.StatusBadRequest)
		return
	}
	var run db.ExecutionRun
	if err := db.DB.First(&run, id).Error; err != nil {
		http.Error(w, "execution run not found", http.StatusNotFound)
		return
	}
	if strings.TrimSpace(run.TopicBranch) == "" || strings.TrimSpace(run.ProjectRef) == "" {
		http.Error(w, "execution run has no GitLab branch", http.StatusConflict)
		return
	}
	client, err := newGitLabDeliveryClient(&s.config.GitLab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	pipeline, err := client.GetLatestPipeline(r.Context(), run.ProjectRef, run.TopicBranch)
	if err != nil {
		_ = recordExecutionAction(run, "refresh_pipeline", authenticatedActor(r), nil, nil, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	run.PipelineStatus = delivery.NormalizeState(pipeline.Status)
	if run.PipelineStatus == "failed" || run.PipelineStatus == "canceled" {
		run.Status = delivery.RunTestsFailed
		run.BlockReason = "GitLab pipeline " + run.PipelineStatus
	} else if run.PipelineStatus == "success" && run.Status == delivery.RunTestsFailed {
		run.Status = delivery.RunReviewPending
		run.BlockReason = ""
	}
	run.UpdatedAt = time.Now()
	if err := db.DB.Save(&run).Error; err != nil {
		http.Error(w, "failed to update pipeline status", http.StatusInternalServerError)
		return
	}
	_ = recordExecutionAction(run, "refresh_pipeline", authenticatedActor(r), map[string]string{"ref": run.TopicBranch}, pipeline, nil)
	writeJSON(w, http.StatusOK, map[string]interface{}{"run": executionRunWithActions(run), "pipeline": pipeline})
}

func (s *Server) handleVerifyExecutionRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil {
		http.Error(w, "invalid execution run id", http.StatusBadRequest)
		return
	}
	var req verifyExecutionRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var run db.ExecutionRun
	if err := db.DB.First(&run, id).Error; err != nil {
		http.Error(w, "execution run not found", http.StatusNotFound)
		return
	}
	var contract db.ReviewContract
	if err := db.DB.First(&contract, run.ReviewContractID).Error; err != nil {
		http.Error(w, "review contract not found", http.StatusConflict)
		return
	}
	actor := authenticatedActor(r)
	actorID := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	if !strings.EqualFold(actor, contract.AcceptanceOwner) && !strings.EqualFold(actorID, contract.AcceptanceOwner) {
		http.Error(w, "only the review contract acceptance owner can verify this run", http.StatusForbidden)
		return
	}
	decision := delivery.NormalizeState(req.Decision)
	if decision != "accepted" && decision != "rejected" {
		http.Error(w, "decision must be accepted or rejected", http.StatusBadRequest)
		return
	}
	if decision == "accepted" && run.PipelineStatus != "success" {
		http.Error(w, "a successful GitLab pipeline is required before acceptance", http.StatusConflict)
		return
	}
	now := time.Now()
	run.AcceptanceState = decision
	if decision == "rejected" {
		run.Status = delivery.RunRejected
		run.BlockReason = firstNonBlank(strings.TrimSpace(req.Note), "rejected by business acceptance owner")
		run.CompletedAt = &now
	} else if run.MRState == "merged" {
		run.Status = delivery.RunDelivered
		run.BlockReason = ""
		run.CompletedAt = &now
	} else {
		run.Status = delivery.RunAcceptancePending
		run.BlockReason = "human acceptance complete; waiting for MR merge"
	}
	run.UpdatedAt = now
	if err := db.DB.Save(&run).Error; err != nil {
		http.Error(w, "failed to save acceptance decision", http.StatusInternalServerError)
		return
	}
	_ = recordExecutionAction(run, "human_acceptance", actor, req, map[string]string{"status": run.Status, "acceptance_state": run.AcceptanceState}, nil)
	if run.Status == delivery.RunDelivered {
		_ = markDemandDelivered(run.DemandID, now)
	}
	if run.Status == delivery.RunDelivered || run.Status == delivery.RunRejected {
		_, _ = delivery.EnsureCorpusCandidates(db.DB, run)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"run": executionRunWithActions(run)})
}

func (s *Server) failExecutionRun(run *db.ExecutionRun, action string, err error) {
	run.Status = delivery.RunExecutionFailed
	run.BlockReason = err.Error()
	run.UpdatedAt = time.Now()
	_ = db.DB.Save(run).Error
	_ = recordExecutionAction(*run, action, run.ServiceIdentity, nil, nil, err)
}

func recordExecutionAction(run db.ExecutionRun, action, actor string, request, result interface{}, actionErr error) error {
	now := time.Now()
	requestJSON := encodeJSON(request)
	resultJSON := encodeJSON(result)
	hash := sha256.Sum256([]byte(requestJSON))
	status := "completed"
	errorMessage := ""
	if actionErr != nil {
		status = "failed"
		errorMessage = actionErr.Error()
	}
	entry := db.ExecutionAction{
		ExecutionRunID: run.ID, ActionKey: fmt.Sprintf("%s:%s:%d", run.RunKey, action, now.UnixNano()), Action: action, Status: status,
		Actor: actor, RequestHash: hex.EncodeToString(hash[:]), RequestJSON: requestJSON, ResultJSON: resultJSON,
		ErrorMessage: errorMessage, CreatedAt: now,
	}
	return db.DB.Create(&entry).Error
}

func executionRunWithActions(run db.ExecutionRun) executionRunDTO {
	actions := []db.ExecutionAction{}
	_ = db.DB.Where("execution_run_id = ?", run.ID).Order("created_at asc").Find(&actions).Error
	return executionRunDTO{ExecutionRun: run, Actions: actions}
}

func executionRunKey(req executionPreflightRequest) string {
	normalized := struct {
		SpecID       uint
		Repo         string
		ChangeSet    []delivery.FileAction
		TestCommands []string
	}{req.DemandSpecVersionID, strings.ToLower(strings.TrimSpace(req.Repo)), req.ChangeSet, normalizeDeliveryStrings(req.TestCommands)}
	digest := sha256.Sum256([]byte(encodeJSON(normalized)))
	return hex.EncodeToString(digest[:16])
}

var branchSlugPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func buildTopicBranch(demandID string, version int, runKey string) string {
	slug := strings.ToLower(strings.TrimSpace(demandID))
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = branchSlugPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	if slug == "" {
		slug = "demand"
	}
	return fmt.Sprintf("ai/%s/v%d-%s", slug, version, runKey[:8])
}

func buildDraftMRDescription(run db.ExecutionRun, spec db.DemandSpecVersion) string {
	acceptance := decodeStringList(spec.AcceptanceCriteriaJSON)
	tests := decodeStringList(run.TestCommandsJSON)
	return fmt.Sprintf("## Controlled autonomous delivery\n\n- Demand: `%s`\n- Frozen spec: v%d (`%d`)\n- Context pack: `%d`\n- Execution run: `%s`\n- Service identity: `%s`\n\n### Acceptance criteria\n%s\n\n### Required tests\n%s\n\n> This MR is intentionally Draft and requires human code review and business acceptance.\n", run.DemandID, spec.Version, spec.ID, spec.ContextPackID, run.RunKey, run.ServiceIdentity, markdownList(acceptance), markdownList(tests))
}

func markdownList(items []string) string {
	if len(items) == 0 {
		return "- Not provided"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func (s *Server) recordExecutionEvidence(run db.ExecutionRun, commit gitLabCommit) error {
	now := time.Now()
	logs := []db.GitCommitLog{
		{TaskID: run.DemandID, Repo: run.Repo, Branch: run.TopicBranch, Action: "ai_branch", Author: run.ServiceIdentity, CreatedAt: now},
		{TaskID: run.DemandID, Repo: run.Repo, Branch: run.TopicBranch, CommitID: commit.ID, Message: "approved autonomous delivery", Action: "git_push", Author: run.ServiceIdentity, CreatedAt: now},
		{TaskID: run.DemandID, Repo: run.Repo, Branch: run.TopicBranch, CommitID: commit.ID, MrIID: run.MRIID, MrURL: run.MRURL, Message: "Draft MR created", Action: "mr_open", Author: run.ServiceIdentity, CreatedAt: now},
	}
	tx := db.DB.Begin()
	for _, logEntry := range logs {
		if err := tx.Create(&logEntry).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	result := tx.Model(&db.TaskTelemetry{}).Where("task_id = ?", run.DemandID).Updates(map[string]interface{}{
		"branch": run.TopicBranch, "mr_i_id": run.MRIID, "mr_url": run.MRURL, "status": "review", "last_update": now,
	})
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("demand %s not found for evidence projection", run.DemandID)
	}
	return tx.Commit().Error
}

func markDemandDelivered(demandID string, now time.Time) error {
	return db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", demandID).Updates(map[string]interface{}{
		"status": "done", "completed_at": &now, "last_update": now,
	}).Error
}
