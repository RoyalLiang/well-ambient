package server

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"net/http"
	"strconv"
	"strings"
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
		"repository_source_json": run.RepositorySourceJSON,
		"id":                     run.ID,
		"project_id":             run.ProjectID,
		"repo":                   run.Repo,
		"author":                 run.Author,
		"kind":                   run.Kind,
		"ref":                    run.Ref,
		"head_sha":               run.HeadSHA,
		"base_sha":               run.BaseSHA,
		"title":                  run.Title,
		"url":                    run.URL,
		"status":                 run.Status,
		"phase":                  run.Phase,
		"error":                  run.Error,
		"model":                  run.Model,
		"prompt_version":         run.PromptVersion,
		"skill_version_id":       run.SkillVersionID,
		"skill_version":          run.SkillVersion,
		"skill_scope_type":       run.SkillScopeType,
		"skill_scope_id":         run.SkillScopeID,
		"skill_name":             run.SkillName,
		"skill_hash":             run.SkillHash,
		"retry_of_id":            run.RetryOfID,
		"publish_status":         run.PublishStatus,
		"publish_error":          run.PublishError,
		"comment_id":             run.CommentID,
		"published_at":           run.PublishedAt,
		"created_at":             run.CreatedAt,
		"updated_at":             run.UpdatedAt,
	}
}
func (s *Server) handleCodeReviewRepos(w http.ResponseWriter, r *http.Request) {
	type repoDTO struct {
		NamespaceName string              `json:"namespace_name"`
		NamespacePath string              `json:"namespace_path"`
		ProjectID     string              `json:"project_id"`
		Name          string              `json:"name"`
		Policy        db.CodeReviewPolicy `json:"policy"`
	}
	repos := []repoDTO{}
	var policies []db.CodeReviewPolicy
	if err := s.codeReview.DB.Find(&policies).Error; err != nil {
		http.Error(w, "无法读取评审配置", 500)
		return
	}
	byProject := make(map[string]db.CodeReviewPolicy, len(policies))
	for _, policy := range policies {
		byProject[policy.ProjectID] = policy
	}
	for _, repo := range s.codeReview.Config().GitLab.Repos {
		if repo.ProjectID == "" {
			continue
		}
		p, exists := byProject[repo.ProjectID]
		if !exists {
			p.ProjectID = repo.ProjectID
		}
		p = codereview.SystemCorpusPolicy(p)
		repos = append(repos, repoDTO{ProjectID: repo.ProjectID, Name: repo.DisplayName(), Policy: p, NamespaceName: repo.NamespaceName, NamespacePath: repo.NamespacePath})
	}
	settings, err := s.codeReview.CommentSettings(r.Context())
	if err != nil {
		http.Error(w, "无法读取全局评论设置", http.StatusInternalServerError)
		return
	}
	emailJSON(w, 200, map[string]any{"repos": repos, "dimensions": codereview.Dimensions, "scenarios": codereview.Scenarios, "default_rules": codereview.DefaultReviewRules, "comment_settings": settings})
}

func (s *Server) handleGetCodeReviewCommentSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.codeReview.CommentSettings(r.Context())
	if err != nil {
		http.Error(w, "无法读取全局评论设置", http.StatusInternalServerError)
		return
	}
	emailJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePatchCodeReviewCommentSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReviewCommits *bool `json:"review_commits"`
		ReviewMRs     *bool `json:"review_mrs"`
		SyncCommits   *bool `json:"sync_commits"`
		SyncMRs       *bool `json:"sync_mrs"`
	}
	if err := decodeEmailRequest(w, r, &req); err != nil || (req.SyncCommits == nil && req.SyncMRs == nil && req.ReviewCommits == nil && req.ReviewMRs == nil) {
		http.Error(w, "请提供有效的全局评论开关", http.StatusBadRequest)
		return
	}
	settings, err := s.codeReview.PatchCommentSettings(r.Context(), req.SyncCommits, req.SyncMRs, req.ReviewCommits, req.ReviewMRs)
	if err != nil {
		http.Error(w, "全局评论设置保存失败", http.StatusInternalServerError)
		return
	}
	emailJSON(w, http.StatusOK, settings)
}
func (s *Server) handleSaveCodeReviewPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID      string  `json:"project_id"`
		ReviewCommits  *bool   `json:"review_commits"`
		ReviewMRs      *bool   `json:"review_mrs"`
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
	if req.ReviewCommits != nil {
		p.ReviewCommits = req.ReviewCommits
	}
	if req.ReviewMRs != nil {
		p.ReviewMRs = req.ReviewMRs
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
	// Scope is fixed even for older clients submitting legacy narrowing fields.
	p = codereview.SystemCorpusPolicy(p)
	if err := codereview.ValidatePolicy(p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	columns := []string{"AutoReview", "Domain", "Scenario", "KnowledgeScope", "UpdatedAt"}
	if req.ReviewCommits != nil {
		columns = append(columns, "ReviewCommits")
	}
	if req.ReviewMRs != nil {
		columns = append(columns, "ReviewMRs")
	}
	if req.SyncCommits != nil {
		columns = append(columns, "SyncCommits")
	}
	if req.SyncMRs != nil {
		columns = append(columns, "SyncMRs")
	}
	if req.Rules != nil {
		columns = append(columns, "Rules")
	}
	for i, column := range columns {
		if column == "ReviewMRs" {
			columns[i] = db.CodeReviewMRsColumn
		} else {
			columns[i] = db.DB.NamingStrategy.ColumnName("", column)
		}
	}
	p.UpdatedAt = time.Now()
	if err := db.DB.WithContext(r.Context()).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: db.DB.NamingStrategy.ColumnName("", "ProjectID")}}, DoUpdates: clause.AssignmentColumns(columns)}).Create(&p).Error; err != nil {
		http.Error(w, "评审配置保存失败", 500)
		return
	}
	emailJSON(w, 200, p)
}
func (s *Server) handleListCodeReviews(w http.ResponseWriter, r *http.Request) {
	project := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if project != "" {
		if _, err := s.codeReview.Repo(project); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	limit, offset := 50, 0
	for name, dest := range map[string]*int{"limit": &limit, "offset": &offset} {
		if value := r.URL.Query().Get(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || (name == "limit" && (n == 0 || n > 200)) {
				http.Error(w, "分页参数无效", 400)
				return
			}
			*dest = n
		}
	}
	kind, query := r.URL.Query().Get("kind"), r.URL.Query().Get("q")
	if (kind != "" && kind != "mr" && kind != "commit") || len([]rune(query)) > 200 {
		http.Error(w, "类型或搜索参数无效", 400)
		return
	}
	page, err := s.codeReview.Search(r.Context(), codereview.RunFilter{Project: project, Kind: kind, Query: query, Limit: limit, Offset: offset})
	if err != nil {
		http.Error(w, "读取评审列表失败", 500)
		return
	}
	emailJSON(w, 200, page)
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
	response := struct {
		db.CodeReviewRun
		Document *codereview.ReviewDocument `json:"document,omitempty"`
	}{CodeReviewRun: run}
	if document, err := codereview.BuildDocument(run); err == nil {
		response.Document = &document
	}
	emailJSON(w, 200, response)
}
func (s *Server) handleExportCodeReview(w http.ResponseWriter, r *http.Request) {
	run, err := s.reviewFromRequest(r)
	if err != nil {
		http.Error(w, "评审不存在或仓库未配置", 404)
		return
	}
	if run.Status != "completed" && run.Status != "partial" {
		http.Error(w, "评审尚未生成结果，请在评审完成后导出", http.StatusConflict)
		return
	}
	var report codereview.Report
	if json.Unmarshal([]byte(run.ReportJSON), &report) != nil || strings.TrimSpace(report.Summary) == "" {
		http.Error(w, "评审结果无法解析，请核对原始记录", http.StatusConflict)
		return
	}
	document, err := codereview.BuildDocument(run)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+document.Filename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(document.Markdown))
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
			if settings, err := s.codeReview.CommentSettings(r.Context()); err == nil {
				if run.Kind == "mr" && !settings.SyncMRs {
					message = "全局 MR 评论同步已关闭"
				} else if run.Kind == "commit" && !settings.SyncCommits {
					message = "全局 Commit 评论同步已关闭"
				}
			}
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
		if repo.InstanceID == "" && other.InstanceID == "" && other.ProjectID != repo.ProjectID && other.Name == repo.Name {
			emailJSON(w, 200, map[string]any{"items": []any{}, "notice": "仓库名称存在重复映射，请输入完整 SHA 或 MR 编号"})
			return
		}
	}
	var logs []db.GitCommitLog
	query := db.DB.Where("action IN ?", []string{"git_push", "mr_open", "mr_update", "mr_reopen", "mr_merge", "mr_close"})
	key := s.codeReview.Config().GitLab.Source(repo, time.Now()).RepositoryKey
	if repo.InstanceID != "" {
		query = query.Where("repository_key = ?", key)
	} else {
		query = query.Where("repository_key = ? OR ((repository_key IS NULL OR repository_key = '') AND repo = ?)", key, repo.Name)
	}
	if err = query.Order("created_at DESC, id DESC").Limit(100).Find(&logs).Error; err != nil {
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
