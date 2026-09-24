package server

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
	"time"
	"well-ambient/internal/codereview"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func (s *Server) newCodeReviewService() *codereview.Service {
	return &codereview.Service{DB: db.DB, Config: func() *config.Config { c := s.currentEmailConfig(); return &c }}
}
func codeReviewActionResponse(run db.CodeReviewRun) map[string]any {
	return map[string]any{
		"id":               run.ID,
		"project_id":       run.ProjectID,
		"repo":             run.Repo,
		"author":           run.Author,
		"kind":             run.Kind,
		"ref":              run.Ref,
		"head_sha":         run.HeadSHA,
		"base_sha":         run.BaseSHA,
		"title":            run.Title,
		"url":              run.URL,
		"status":           run.Status,
		"phase":            run.Phase,
		"error":            run.Error,
		"model":            run.Model,
		"prompt_version":   run.PromptVersion,
		"skill_version_id": run.SkillVersionID,
		"skill_version":    run.SkillVersion,
		"skill_scope_type": run.SkillScopeType,
		"skill_scope_id":   run.SkillScopeID,
		"skill_name":       run.SkillName,
		"skill_hash":       run.SkillHash,
		"retry_of_id":      run.RetryOfID,
		"publish_status":   run.PublishStatus,
		"publish_error":    run.PublishError,
		"comment_id":       run.CommentID,
		"published_at":     run.PublishedAt,
		"created_at":       run.CreatedAt,
		"updated_at":       run.UpdatedAt,
	}
}
func (s *Server) handleCodeReviewRepos(w http.ResponseWriter, r *http.Request) {
	type repoDTO struct {
		ProjectID string              `json:"project_id"`
		Name      string              `json:"name"`
		Policy    db.CodeReviewPolicy `json:"policy"`
	}
	repos := []repoDTO{}
	for _, repo := range s.codeReview.Config().GitLab.Repos {
		if repo.ProjectID == "" {
			continue
		}
		p, err := s.codeReview.Policy(repo.ProjectID)
		if err != nil {
			http.Error(w, "无法读取评审配置", 500)
			return
		}
		repos = append(repos, repoDTO{repo.ProjectID, repo.Name, p})
	}
	emailJSON(w, 200, map[string]any{"repos": repos, "dimensions": codereview.Dimensions, "scenarios": codereview.Scenarios, "default_rules": codereview.DefaultReviewRules})
}
func (s *Server) handleSaveCodeReviewPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID      string  `json:"project_id"`
		SyncCommits    *bool   `json:"sync_commits"`
		SyncMRs        *bool   `json:"sync_mrs"`
		Domain         *string `json:"domain"`
		Scenario       *string `json:"scenario"`
		KnowledgeScope *string `json:"knowledge_scope"`
		Rules          *string `json:"rules"`
	}
	if err := decodeEmailRequest(w, r, &req); err != nil {
		http.Error(w, "评审配置格式无效", 400)
		return
	}
	if _, err := s.codeReview.Repo(req.ProjectID); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	unlockPolicy := codereview.LockPolicy(req.ProjectID)
	defer unlockPolicy()
	p, err := s.codeReview.Policy(req.ProjectID)
	if err != nil {
		http.Error(w, "无法读取评审配置", 500)
		return
	}
	if req.SyncCommits != nil {
		p.SyncCommits = *req.SyncCommits
	}
	if req.SyncMRs != nil {
		p.SyncMRs = *req.SyncMRs
	}
	if req.Domain != nil {
		p.Domain = *req.Domain
	}
	if req.Scenario != nil {
		p.Scenario = *req.Scenario
	}
	if req.KnowledgeScope != nil {
		p.KnowledgeScope = *req.KnowledgeScope
	}
	if req.Rules != nil {
		p.Rules = *req.Rules
	}
	p.AutoReview = true // New webhook events always trigger review.
	if err := codereview.ValidatePolicy(p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := db.DB.WithContext(r.Context()).Save(&p).Error; err != nil {
		http.Error(w, "评审配置保存失败", 500)
		return
	}
	emailJSON(w, 200, p)
}
func (s *Server) handleListCodeReviews(w http.ResponseWriter, r *http.Request) {
	runs, err := s.codeReview.List(200)
	if err != nil {
		http.Error(w, "读取评审列表失败", 500)
		return
	}
	emailJSON(w, 200, map[string]any{"items": runs})
}
func (s *Server) handleCreateCodeReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		Kind      string `json:"kind"`
		Ref       string `json:"ref"`
	}
	if err := decodeEmailRequest(w, r, &req); err != nil {
		http.Error(w, "请求格式无效", 400)
		return
	}
	run, err := s.codeReview.Enqueue(r.Context(), req.ProjectID, req.Kind, req.Ref, "")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	emailJSON(w, 202, run)
}
func (s *Server) handleGetCodeReview(w http.ResponseWriter, r *http.Request) {
	run, err := s.reviewFromRequest(r)
	if err != nil {
		http.Error(w, "评审不存在或仓库未配置", 404)
		return
	}
	emailJSON(w, 200, run)
}
func (s *Server) reviewFromRequest(r *http.Request) (db.CodeReviewRun, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil {
		return db.CodeReviewRun{}, err
	}
	return s.codeReview.Get(uint(id))
}
func (s *Server) handleCancelCodeReview(w http.ResponseWriter, r *http.Request) {
	run, err := s.reviewFromRequest(r)
	if err != nil {
		http.Error(w, "评审不存在", 404)
		return
	}
	if err = s.codeReview.Cancel(run.ID); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	run, _ = s.codeReview.Get(run.ID)
	emailJSON(w, 200, codeReviewActionResponse(run))
}
func (s *Server) handleRetryCodeReview(w http.ResponseWriter, r *http.Request) {
	run, err := s.reviewFromRequest(r)
	if err != nil {
		http.Error(w, "评审不存在", 404)
		return
	}
	retry, err := s.codeReview.Retry(r.Context(), run.ID)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	emailJSON(w, 202, codeReviewActionResponse(retry))
}
func (s *Server) handleSyncCodeReview(w http.ResponseWriter, r *http.Request) {
	run, err := s.reviewFromRequest(r)
	if err != nil {
		http.Error(w, "评审不存在", 404)
		return
	}
	if run.Status != "completed" {
		http.Error(w, "仅完整评审可以同步；请先处理上下文缺口", 409)
		return
	}
	// This explicit action still respects switches and never retries an unknown POST.
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err = s.codeReview.Publish(ctx, run); err != nil {
		s.codeReview.RecordPublishError(run.ID, err)
		http.Error(w, err.Error(), 409)
		return
	}
	run, _ = s.codeReview.Get(run.ID)
	if run.PublishStatus != "published" {
		message := "评论同步未执行"
		switch run.PublishStatus {
		case "off":
			message = "当前仓库的评论同步开关已关闭"
		case "stale":
			message = "代码版本已变化，不能同步旧评审"
		case "unknown":
			message = "评论发送结果仍待核对"
		case "blocked":
			message = "评审证据不完整，评论同步受阻"
		case "sync_failed":
			message = firstNonBlank(run.PublishError, "评论同步失败")
		}
		http.Error(w, message, http.StatusConflict)
		return
	}
	emailJSON(w, 200, codeReviewActionResponse(run))
}
func (s *Server) startCodeReviewWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if _, err := s.codeReview.ProcessOne(ctx); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Code review worker: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		if err := s.codeReview.PublishNext(ctx); err != nil {
			log.Printf("Code review comment sync: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) handleCodeReviewTargets(w http.ResponseWriter, r *http.Request) {
	repo, err := s.codeReview.Repo(r.URL.Query().Get("project_id"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// Legacy telemetry identifies repositories by name. Never guess an ambiguous project.
	for _, other := range s.codeReview.Config().GitLab.Repos {
		if other.ProjectID != repo.ProjectID && other.Name == repo.Name {
			emailJSON(w, 200, map[string]any{"items": []any{}, "notice": "仓库名称存在重复映射，请输入完整 SHA 或 MR 编号"})
			return
		}
	}
	var logs []db.GitCommitLog
	if err = db.DB.Where("repo = ? AND action IN ?", repo.Name, []string{"git_push", "mr_open", "mr_update", "mr_reopen", "mr_merge", "mr_close"}).Order("created_at DESC, id DESC").Limit(100).Find(&logs).Error; err != nil {
		http.Error(w, "读取已采集变更失败", 500)
		return
	}
	type target struct {
		Kind   string `json:"kind"`
		Ref    string `json:"ref"`
		Title  string `json:"title"`
		Author string `json:"author"`
		Branch string `json:"branch"`
	}
	items := []target{}
	seen := map[string]bool{}
	for _, entry := range logs {
		kind, ref := "commit", entry.CommitID
		if entry.Action != "git_push" {
			kind, ref = "mr", strconv.Itoa(entry.MrIID)
			if entry.MrIID <= 0 {
				continue
			}
		}
		if ref == "" || seen[kind+":"+ref] {
			continue
		}
		seen[kind+":"+ref] = true
		items = append(items, target{Kind: kind, Ref: ref, Title: entry.Message, Author: entry.Author, Branch: entry.Branch})
	}
	emailJSON(w, 200, map[string]any{"items": items})
}
