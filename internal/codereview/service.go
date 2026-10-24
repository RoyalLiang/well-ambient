package codereview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

type Service struct {
	DB       *gorm.DB
	Config   func() *config.Config
	HTTP     *http.Client
	Generate func(context.Context, string, string) (string, error)

	// Package-local intervals let lifetime tests exercise the real monitor.
	reviewPollInterval      time.Duration
	reviewHeartbeatInterval time.Duration
}

var policyLocks sync.Map

func LockPolicy(project string) func() {
	value, _ := policyLocks.LoadOrStore(project, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (s *Service) git() GitLab { return GitLab{Config: s.Config().GitLab, Client: s.HTTP} }
func (s *Service) Repo(project string) (config.RepoMapping, error) {
	for _, r := range s.Config().GitLab.Repos {
		if r.ProjectID == project && project != "" {
			return r, nil
		}
	}
	return config.RepoMapping{}, errors.New("仓库未配置，拒绝评审或评论")
}
func (s *Service) Policy(project string) (db.CodeReviewPolicy, error) {
	p := SystemCorpusPolicy(db.CodeReviewPolicy{ProjectID: project})
	err := s.DB.Where("project_id = ?", project).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SystemCorpusPolicy(p), nil
	}
	// The governance contract uses the system corpus; legacy narrowing fields
	// are retained in storage for audit but do not restrict new reviews.
	return SystemCorpusPolicy(p), err
}
func (s *Service) ActiveReviewSkill(ctx context.Context) (db.SolutionPromptTemplate, error) {
	var skill db.SolutionPromptTemplate
	query := s.DB.WithContext(ctx).
		Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ? AND validation_status = ?", "code_review", "global", "", "active", "passed").
		Order("version DESC").Limit(1).Find(&skill)
	if query.Error != nil {
		return skill, query.Error
	}
	if query.RowsAffected == 0 {
		return skill, errors.New("线上代码评审技能未配置或未启用")
	}
	return skill, nil
}
func (s *Service) ReviewSkillByID(ctx context.Context, id uint) (db.SolutionPromptTemplate, error) {
	var skill db.SolutionPromptTemplate
	if id == 0 {
		return skill, errors.New("代码评审技能版本为空")
	}
	err := s.DB.WithContext(ctx).
		Where("id = ? AND purpose = ?", id, "code_review").
		First(&skill).Error
	return skill, err
}
func (s *Service) Enqueue(ctx context.Context, project, kind, ref, eventKey string) (db.CodeReviewRun, error) {
	return s.enqueue(ctx, project, kind, ref, eventKey, nil)
}

func (s *Service) enqueue(ctx context.Context, project, kind, ref, eventKey string, expected *mrCodeVersion) (db.CodeReviewRun, error) {
	var run db.CodeReviewRun
	repo, err := s.Repo(project)
	if err != nil {
		return run, err
	}
	if kind != "commit" && kind != "mr" {
		return run, errors.New("仅支持 commit 或 mr")
	}
	if (kind == "commit" && !shaPattern.MatchString(ref)) || (kind == "mr" && !safeRef.MatchString(ref)) {
		return run, errors.New("Commit 需要完整 SHA；MR 需要正整数 IID")
	}
	connection, _, connectionErr := s.Config().GitLab.ForRepository(project)
	if connectionErr != nil {
		return run, connectionErr
	}
	if !connection.Enabled || connection.APIToken == "" || !s.Config().AI.Enabled {
		return run, errors.New("请先配置并启用 GitLab 和 AI")
	}
	p, err := s.Policy(project)
	if err != nil {
		return run, err
	}
	if err = ValidatePolicy(p); err != nil {
		return run, err
	}
	global, gateErr := s.CommentSettings(ctx)
	if gateErr != nil {
		return run, gateErr
	}
	if (kind == "mr" && (!global.ReviewMRs || !*p.ReviewMRs)) || (kind == "commit" && (!global.ReviewCommits || !*p.ReviewCommits)) {
		return run, fmt.Errorf("当前已暂停 %s 评审", kind)
	}

	skill, err := s.ActiveReviewSkill(ctx)
	if err != nil {
		return run, err
	}
	method, err := s.freezeReviewMethod(ctx)
	if err != nil {
		return run, err
	}
	if eventKey == "" {
		b := make([]byte, 16)
		if _, err = rand.Read(b); err != nil {
			return run, err
		}
		eventKey = hex.EncodeToString(b)
	}
	publishStatus := "off"
	allowed, err := s.commentsAllowed(ctx, kind, p)
	if err != nil {
		return run, err
	}
	if allowed {
		publishStatus = "waiting_review"
	}
	source := s.Config().GitLab.Source(repo, time.Now())
	identity := source.RepositoryKey
	if source.LegacyDedupe {
		identity = project
	}
	run = db.CodeReviewRun{
		Key: digest(identity + ":" + kind + ":" + ref + ":" + eventKey), ProjectID: project, Repo: repo.DisplayName(),
		RepositorySourceJSON: source.JSON(),
		Kind:                 kind, Ref: ref, Status: "queued", Phase: "等待执行", PolicyJSON: encode(frozenReviewPolicy{CodeReviewPolicy: p, Method: method}),
		PromptVersion:  fmt.Sprintf("review-skill:%d/v%d@%s", skill.ID, skill.Version, RuntimePromptVersion),
		SkillVersionID: skill.ID, SkillVersion: skill.Version, SkillScopeType: skill.ScopeType,
		SkillScopeID: skill.ScopeID, SkillName: skill.Name, SkillHash: digest(skill.SystemPrompt),
		Model: s.Config().AI.Model, PublishStatus: publishStatus,
	}
	if expected != nil {
		run.HeadSHA = expected.Head
		run.BaseSHA = expected.Base
	}
	res := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
	if res.Error != nil {
		return run, res.Error
	}
	if res.RowsAffected == 0 {
		err = s.DB.Where("key = ?", run.Key).First(&run).Error
	}
	return run, err
}
func (s *Service) Hook(ctx context.Context, event string, body []byte) error {
	var p struct {
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		After   string `json:"after"`
		Ref     string `json:"ref"`
		Commits []struct {
			ID string `json:"id"`
		} `json:"commits"`
		Attr struct {
			IID     int    `json:"iid"`
			State   string `json:"state"`
			Updated string `json:"updated_at"`
			Last    struct {
				ID string `json:"id"`
			} `json:"last_commit"`
		} `json:"object_attributes"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	id := strconv.Itoa(p.Project.ID)
	if instance := s.Config().GitLab.ActiveInstanceID; instance != "" {
		id = instance + ":" + id
	}
	if _, err := s.Repo(id); err != nil {
		return nil
	}
	var err error
	policy, err := s.Policy(id)
	if err != nil {
		return err
	}
	global, err := s.CommentSettings(ctx)
	if err != nil {
		return err
	}
	reviewMRs := global.ReviewMRs && *policy.ReviewMRs
	reviewCommits := global.ReviewCommits && *policy.ReviewCommits
	if event == "Push Hook" {
		branch := ""
		if strings.HasPrefix(p.Ref, "refs/heads/") {
			branch = strings.TrimPrefix(p.Ref, "refs/heads/")
		}
		var openMRs []mr
		if reviewMRs && branch != "" {
			openMRs, err = s.git().OpenMRsForBranch(ctx, id, branch)
			if err != nil {
				return err
			}
		}
		if len(openMRs) > 0 {
			for _, m := range openMRs {
				version := mrVersion(m)
				if version.Head == "" || version.Base == "" {
					current, fetchErr := s.git().mr(ctx, id, strconv.Itoa(m.IID))
					if fetchErr != nil {
						return fetchErr
					}
					version = mrVersion(current)
				}
				if _, err = s.enqueueMRVersion(ctx, id, strconv.Itoa(m.IID), version); err != nil {
					return err
				}
			}
		}
		if reviewCommits {
			for _, c := range p.Commits {
				if _, err = s.Enqueue(ctx, id, "commit", c.ID, "push:"+c.ID); err != nil {
					return err
				}
			}
		}
	} else if event == "Merge Request Hook" && p.Attr.State == "opened" && reviewMRs {
		current, fetchErr := s.git().mr(ctx, id, strconv.Itoa(p.Attr.IID))
		if fetchErr != nil {
			return fetchErr
		}
		_, err = s.enqueueMRVersion(ctx, id, strconv.Itoa(p.Attr.IID), mrVersion(current))
	}
	return err
}
func (s *Service) generate(ctx context.Context, ai config.AIConfig, system, user string) (string, error) {
	return s.generateObserved(ctx, ai, system, user, nil)
}

type SkillValidationResult struct {
	Passed           bool   `json:"passed"`
	RuntimeVersion   string `json:"runtime_version"`
	Dimensions       int    `json:"dimensions"`
	FindingsRetained int    `json:"findings_retained"`
	Questions        int    `json:"questions"`
	EvidenceComplete bool   `json:"evidence_complete"`
	Summary          string `json:"summary"`
}

func (s *Service) ValidateReviewSkill(ctx context.Context, skillPrompt string) (SkillValidationResult, error) {
	snapshot := Snapshot{
		Author: "skill-validator", Head: strings.Repeat("a", 40), Base: strings.Repeat("b", 40),
		Title: "校验车辆反馈完成边界", Description: "空反馈不得被当作完成；需要匹配任务与车辆的反馈证据。",
		Complete: true, Gaps: []string{},
		Knowledge: []Knowledge{{
			ID: 1, Version: 1, Scope: "global", Source: "skill-validator",
			Summary: "任务完成证据", Content: "空反馈不代表完成；任务和车辆身份必须匹配。",
		}},
		Files: []File{{
			NewPath: "dispatch/completion.go",
			Diff:    "@@ -1,4 +1,4 @@\n package dispatch\n func completeTask(feedback string) bool {\n- return feedback == \"completed\"\n+ return feedback == \"\"\n }",
			Source:  "package dispatch\nfunc completeTask(feedback string) bool {\n return feedback == \"\"\n}",
		}},
	}
	policy := db.CodeReviewPolicy{Domain: "general", Scenario: "general"}
	system := ReviewPrompt(skillPrompt, policy)
	input := encode(snapshot)
	ai := s.Config().AI
	raw, err := s.generate(ctx, ai, system, input)
	if err != nil {
		return SkillValidationResult{}, err
	}
	draft, err := parseReport(raw, snapshot)
	if err != nil {
		return SkillValidationResult{}, err
	}
	raw, err = s.generate(
		ctx,
		ai,
		system+"\n本轮是独立复核。先从代码重新判断，再核对初审。主动寻找反证，删除不成立或已经被其他代码保护的问题；不能把初审当作证据。保留全部维度分析，输出相同 JSON 结构。",
		input+"\n<untrusted_draft>"+encode(draft)+"</untrusted_draft>",
	)
	if err != nil {
		return SkillValidationResult{}, err
	}
	report, err := parseReport(raw, snapshot)
	if err != nil {
		return SkillValidationResult{}, err
	}
	if !report.EvidenceComplete {
		return SkillValidationResult{}, errors.New("代码评审技能 dry-run 未通过证据校验")
	}
	return SkillValidationResult{
		Passed: true, RuntimeVersion: RuntimePromptVersion, Dimensions: len(report.Assessments),
		FindingsRetained: len(report.Findings), Questions: len(report.Questions),
		EvidenceComplete: report.EvidenceComplete, Summary: report.Summary,
	}, nil
}
func (s *Service) ProcessOne(ctx context.Context) (bool, error) {
	// An interrupted run is visible as failed; it is never silently treated as successful.
	if err := s.DB.Model(&db.CodeReviewRun{}).Where("status = ? AND updated_at < ?", "running", time.Now().Add(-15*time.Minute)).Updates(map[string]any{"status": "failed", "phase": "运行中断", "error": "执行超时或进程重启；请重新评审"}).Error; err != nil {
		return false, err
	}
	var run db.CodeReviewRun
	err := s.DB.Order("id ASC").Where("status = ?", "queued").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	currentPolicy, gateErr := s.Policy(run.ProjectID)
	if gateErr != nil {
		return true, gateErr
	}
	global, gateErr := s.CommentSettings(ctx)
	if gateErr != nil {
		return true, gateErr
	}
	if (run.Kind == "mr" && (!global.ReviewMRs || !*currentPolicy.ReviewMRs)) || (run.Kind == "commit" && (!global.ReviewCommits || !*currentPolicy.ReviewCommits)) {
		err := s.DB.Model(&run).Where("status = ?", "queued").Updates(map[string]any{"status": "cancelled", "phase": "该类型评审已暂停", "publish_status": "off"}).Error
		return true, err
	}
	claim := s.DB.Model(&db.CodeReviewRun{}).Where("id = ? AND status = ?", run.ID, "queued").Updates(map[string]any{"status": "running", "phase": "读取代码与知识"})
	if claim.Error != nil || claim.RowsAffected == 0 {
		return false, claim.Error
	}
	// Model generation inherits the caller's lifetime. User cancellation and
	// service shutdown still stop it; a local fixed deadline must not kill it.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.monitorReviewRun(ctx, cancel, run.ID)

	fail := func(e error) (bool, error) {
		var current db.CodeReviewRun
		if s.DB.Select("status").First(&current, run.ID).Error == nil && current.Status == "cancelled" {
			return true, nil
		}
		_ = s.DB.Model(&run).Where("status = ?", "running").Updates(map[string]any{"status": "failed", "phase": "评审失败", "error": e.Error()}).Error
		return true, e
	}
	repo, err := s.Repo(run.ProjectID)
	if err != nil {
		return fail(err)
	}
	var frozen frozenReviewPolicy
	if err = json.Unmarshal([]byte(run.PolicyJSON), &frozen); err != nil {
		return fail(err)
	}
	policy := SystemCorpusPolicy(frozen.CodeReviewPolicy)
	frozen.CodeReviewPolicy = policy
	if frozen.Method == nil {
		frozen.Method, err = s.freezeReviewMethod(ctx)
		if err != nil {
			return fail(err)
		}
	}
	if err := s.reviewMethodEnabled(ctx, frozen.Method); err != nil {
		return fail(err)
	}
	// A queued run has not produced evidence yet. Freeze its actual effective
	// policy now; completed historical reports retain their original snapshot.
	run.PolicyJSON = encode(frozen)
	run.PromptVersion = fmt.Sprintf("review-skill:%d/v%d@%s", run.SkillVersionID, run.SkillVersion, RuntimePromptVersion)
	if err = s.DB.Model(&run).Updates(map[string]any{"policy_json": run.PolicyJSON, "prompt_version": run.PromptVersion}).Error; err != nil {
		return fail(err)
	}
	skillPrompt := db.DefaultCodeReviewSkillPrompt
	if run.SkillVersionID > 0 {
		skill, skillErr := s.ReviewSkillByID(ctx, run.SkillVersionID)
		if skillErr != nil {
			return fail(errors.New("冻结的线上代码评审技能版本不存在"))
		}
		if run.SkillHash != "" && digest(skill.SystemPrompt) != run.SkillHash {
			return fail(errors.New("冻结的线上代码评审技能内容校验失败"))
		}
		skillPrompt = skill.SystemPrompt
	}
	if frozen.Method != nil {
		skillPrompt = frozen.Method.Instructions + "\n<online_policy>\n" + skillPrompt + "\n</online_policy>"
	}
	git := s.git()
	if run.RepositorySourceJSON == "" {
		connection, _, err := git.Config.ForRepository(run.ProjectID)
		if err != nil || connection.ActiveInstanceID != "" || (git.Config.LegacyDedupeOrigin != "" && strings.TrimRight(connection.BaseURL, "/") != strings.TrimRight(git.Config.LegacyDedupeOrigin, "/")) {
			return fail(errors.New("历史评审缺少可验证的仓库来源，请重新发起评审"))
		}
	}
	var frozenSource config.RepositorySource
	if run.RepositorySourceJSON != "" {
		if err := json.Unmarshal([]byte(run.RepositorySourceJSON), &frozenSource); err != nil {
			return fail(errors.New("仓库来源快照无效"))
		}
		if frozenSource.RepositoryKey != git.Config.Source(repo, time.Now()).RepositoryKey {
			return fail(errors.New("仓库来源已变更，请重新发起评审"))
		}
	}
	snapshot, err := git.Snapshot(ctx, run.ProjectID, run.Kind, run.Ref)
	if err != nil {
		return fail(err)
	}
	if versionBoundMR(run) && (snapshot.Head != run.HeadSHA || snapshot.Base != run.BaseSHA) {
		if _, err = s.enqueueMRVersion(ctx, run.ProjectID, run.Ref, mrCodeVersion{Head: snapshot.Head, Base: snapshot.Base}); err != nil {
			return fail(err)
		}
		err = s.DB.Model(&db.CodeReviewRun{}).Where("id = ? AND status = ?", run.ID, "running").Updates(map[string]any{"status": "cancelled", "phase": skippedMRVersionPhase, "publish_status": "off", "error": ""}).Error
		return true, err
	}
	if run.RepositorySourceJSON != "" {
		if err := json.Unmarshal([]byte(run.RepositorySourceJSON), &snapshot.RepositorySource); err != nil {
			return fail(errors.New("仓库来源快照无效"))
		}
	}
	if frozen.Method != nil {
		snapshot.ReviewMethod = &frozen.Method.ReviewMethod
	}
	snapshot.EvidenceContract = &EvidenceContract{Version: RuntimePromptVersion, PromptHash: digest(ReviewPrompt(skillPrompt, policy)), Rules: append([]string{}, DefaultReviewRules...)}
	snapshot.Knowledge, err = s.knowledge(ctx, policy, repo, snapshot)
	if err != nil {
		return fail(errors.New("系统知识库读取失败"))
	}
	if len(snapshot.Knowledge) == 0 {
		snapshot.Gaps = append(snapshot.Gaps, "未检索到适用的已启用系统知识，业务结论需要人工确认")
	}
	if policy.Domain == "fms" {
		scoped := false
		for _, k := range snapshot.Knowledge {
			if strings.HasPrefix(k.Scope, "repo:") || strings.HasPrefix(k.Scope, "project:") {
				scoped = true
			}
		}
		if !scoped {
			snapshot.Complete = false
			snapshot.Gaps = append(snapshot.Gaps, "FMS 评审缺少仓库/项目范围的知识依据")
		}
	}
	if err = s.DB.Model(&run).Updates(map[string]any{"head_sha": snapshot.Head, "base_sha": snapshot.Base, "title": snapshot.Title, "author": snapshot.Author, "url": snapshot.URL, "snapshot_json": encode(snapshot), "phase": "工程与 FMS 场景分析"}).Error; err != nil {
		return fail(err)
	}
	run.HeadSHA = snapshot.Head
	run.BaseSHA = snapshot.Base
	reviewAI := s.Config().AI
	if err = s.DB.Model(&run).Update("model", reviewAI.Model).Error; err != nil {
		return fail(err)
	}
	prompt := ReviewPrompt(skillPrompt, policy)
	report, err := s.reviewSnapshot(ctx, reviewAI, prompt, &snapshot)
	// Persist per-call optimization evidence even when a provider call fails.
	if saveErr := s.DB.Model(&run).Update("snapshot_json", encode(snapshot)).Error; saveErr != nil {
		return fail(saveErr)
	}
	if err != nil {
		return fail(err)
	}
	report.Scenario = Scenarios[policy.Scenario]
	report.ReviewMethod = snapshot.ReviewMethod
	report.Questions = append(report.Questions, snapshot.Gaps...)
	status := "completed"
	if !snapshot.Complete || !report.EvidenceComplete {
		status = "partial"
	}
	unlockPolicy := LockPolicy(run.ProjectID)
	defer unlockPolicy()
	publish := "off"
	current, err := s.Policy(run.ProjectID)
	if err != nil {
		return fail(err)
	}
	allowed, err := s.commentsAllowed(ctx, run.Kind, current)
	if err != nil {
		return fail(err)
	}
	if allowed {
		publish = "pending"
	}
	if status == "partial" {
		publish = "blocked"
	}
	if err = s.DB.Model(&run).Where("status = ?", "running").Updates(map[string]any{"status": status, "phase": "评审完成", "report_json": encode(report), "publish_status": publish}).Error; err != nil {
		return fail(err)
	}
	return true, nil
}
func (s *Service) Cancel(id uint) error {
	res := s.DB.Model(&db.CodeReviewRun{}).Where("id = ? AND status IN ?", id, []string{"queued", "running"}).Updates(map[string]any{"status": "cancelled", "phase": "已取消"})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("仅等待中或执行中的评审可以取消")
	}
	return nil
}
func (s *Service) Retry(ctx context.Context, id uint) (db.CodeReviewRun, error) {
	failed, err := s.Get(id)
	if err != nil {
		return db.CodeReviewRun{}, err
	}
	if failed.Status != "failed" {
		return db.CodeReviewRun{}, errors.New("仅失败的评审可以重新评审")
	}
	res := s.DB.WithContext(ctx).Model(&db.CodeReviewRun{}).
		Where("id = ? AND status = ?", failed.ID, "failed").
		Updates(map[string]any{
			"status":         "queued",
			"phase":          "等待重试",
			"error":          "",
			"publish_status": "waiting_review",
			"publish_error":  "",
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return db.CodeReviewRun{}, res.Error
	}
	if res.RowsAffected == 0 {
		return db.CodeReviewRun{}, errors.New("仅失败的评审可以重新评审")
	}
	return s.Get(id)
}
func (s *Service) List(limit int, kinds ...string) ([]db.CodeReviewRun, error) {
	kind := ""
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	page, err := s.Search(context.Background(), RunFilter{Kind: kind, Limit: limit})
	return page.Items, err
}

func (s *Service) ListForRepo(limit int, project string) ([]db.CodeReviewRun, error) {
	result, err := s.Search(context.Background(), RunFilter{Project: project, Limit: limit})
	return result.Items, err
}

func (s *Service) Get(id uint) (db.CodeReviewRun, error) {
	var run db.CodeReviewRun
	err := s.DB.First(&run, id).Error
	if err == nil {
		_, err = s.Repo(run.ProjectID)
	}
	return run, err
}
func (s *Service) PublishNext(ctx context.Context) error {
	var run db.CodeReviewRun
	err := s.DB.Where("publish_status IN ?", []string{"pending", "publishing"}).Order("id ASC").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = s.Publish(ctx, run)
	if err != nil {
		s.RecordPublishError(run.ID, err)
	}
	return err
}

func (s *Service) RecordPublishError(id uint, err error) {
	var run db.CodeReviewRun
	if s.DB.First(&run, id).Error != nil {
		return
	}
	state := run.PublishStatus
	if state == "pending" || state == "off" {
		state = "sync_failed"
	}
	_ = s.DB.Model(&run).Updates(map[string]any{"publish_status": state, "publish_error": err.Error()}).Error
}
func (s *Service) policyAllows(run db.CodeReviewRun) (bool, error) {
	p, err := s.Policy(run.ProjectID)
	if err != nil {
		return false, err
	}
	allowed, err := s.commentsAllowed(context.Background(), run.Kind, p)
	if err != nil || !allowed {
		return false, err
	}
	var old db.CodeReviewPolicy
	if err = json.Unmarshal([]byte(run.PolicyJSON), &old); err != nil {
		return false, err
	}
	if p.Domain != old.Domain || p.Scenario != old.Scenario || p.KnowledgeScope != old.KnowledgeScope || p.Rules != old.Rules {
		return false, errors.New("仓库评审规则、场景或知识范围已变更；旧报告不可同步，后续新事件将按新策略评审")
	}
	return true, nil
}
func (s *Service) status(run db.CodeReviewRun, status string) error {
	return s.DB.Model(&db.CodeReviewRun{}).Where("id = ?", run.ID).Update("publish_status", status).Error
}
func publicationKey(run db.CodeReviewRun) string {
	identity := run.ProjectID
	var source config.RepositorySource
	if json.Unmarshal([]byte(run.RepositorySourceJSON), &source) == nil && source.RepositoryKey != "" && !source.LegacyDedupe {
		identity = source.RepositoryKey
	}
	return digest(fmt.Sprintf("%s:%s:%s:%s:%s", identity, run.Kind, run.Ref, run.BaseSHA, run.HeadSHA))
}
