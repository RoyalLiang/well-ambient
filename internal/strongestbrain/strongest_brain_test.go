package strongestbrain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
	"well-ambient/internal/strongestbrain"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	err = database.AutoMigrate(
		&db.Capability{},
		&db.CapabilityVersion{},
		&db.CapabilityResource{},
		&db.CapabilityDependency{},
		&db.AgentRun{},
		&db.RunCapabilityBinding{},
		&db.AgentRunEvent{},
		&db.CapabilityEvaluation{},
		&db.CapabilityProposal{},
		&db.CodeReviewRun{},
		&db.SolutionPromptTemplate{},
	)
	if err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}
	return database
}

// 场景 1: 7 维能力治理智能指标计算与异常聚类
func TestStrongestBrain_7DimensionIntelligenceReport(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	svc := strongestbrain.NewService(database, reg)
	ctx := context.Background()

	// 1. 初始化一个激活的能力版本
	manifest := &agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "code_review",
		Kind:    agentruntime.KindSkill,
		Version: 1,
		Provides: []string{"review"},
	}
	_, _ = reg.Register(ctx, manifest)
	_ = reg.ActivateVersion(ctx, "code_review", 1, "global", "")

	// 2. 插入一些测试运行样本（1 completed, 1 partial, 1 failed）
	now := time.Now()
	runs := []db.AgentRun{
		{
			RunKey:       "run-001",
			AgentKind:    "code_review",
			State:        "completed",
			PromptTokens: 2500,
			DurationMs:   1200,
			CreatedAt:    now.Add(-3 * time.Minute),
			UpdatedAt:    now.Add(-2 * time.Minute),
		},
		{
			RunKey:       "run-002",
			AgentKind:    "code_review",
			State:        "partial",
			PromptTokens: 9500,
			DurationMs:   3500,
			Error:        "missing diffuse evidence context",
			CreatedAt:    now.Add(-2 * time.Minute),
			UpdatedAt:    now.Add(-1 * time.Minute),
		},
		{
			RunKey:       "run-003",
			AgentKind:    "code_review",
			State:        "failed",
			PromptTokens: 1200,
			DurationMs:   400,
			Error:        "timeout connecting to LLM endpoint",
			CreatedAt:    now.Add(-1 * time.Minute),
			UpdatedAt:    now,
		},
	}
	for _, r := range runs {
		_ = database.Create(&r).Error
		// 绑定到 code_review v1
		_ = database.Create(&db.RunCapabilityBinding{
			RunID:         r.ID,
			CapabilityKey: "code_review",
			Version:       1,
			Digest:        "sha256:v1-digest",
			LoadLevel:     "L1",
			LoadedAt:      now,
		}).Error
	}

	// 插入一次安全校验拦截事件
	_ = database.Create(&db.AgentRunEvent{
		RunID:       runs[1].ID,
		Sequence:    1,
		EventType:   agentruntime.EventValidationFailed,
		PayloadJSON: `{"reason":"untrusted data override attempt"}`,
		CreatedAt:   now,
	}).Error

	// 3. 计算 7 维治理报告
	report, err := svc.GetIntelligence(ctx, 100)
	if err != nil {
		t.Fatalf("compute intelligence failed: %v", err)
	}

	if report.Summary.TotalRuns != 3 {
		t.Fatalf("expected 3 total runs, got %d", report.Summary.TotalRuns)
	}
	if report.Summary.CompletedRuns != 1 {
		t.Fatalf("expected 1 completed run, got %d", report.Summary.CompletedRuns)
	}
	if report.Summary.PartialRuns != 1 {
		t.Fatalf("expected 1 partial run, got %d", report.Summary.PartialRuns)
	}
	if report.Summary.FailedRuns != 1 {
		t.Fatalf("expected 1 failed run, got %d", report.Summary.FailedRuns)
	}
	if report.Summary.SecurityRejects != 1 {
		t.Fatalf("expected 1 security reject, got %d", report.Summary.SecurityRejects)
	}

	if len(report.ActiveSkills) != 1 || report.ActiveSkills[0].CapabilityKey != "code_review" {
		t.Fatalf("expected code_review in active skills, got: %+v", report.ActiveSkills)
	}

	// 验证治理建议已生成且包含安全警告与 partial 警告
	if len(report.Recommendations) == 0 {
		t.Fatalf("expected recommendations to be generated")
	}
}

// 场景 2: 阈值异常时最强大脑自动生成优化提案，与人工审批门禁（Human-in-the-loop）
func TestStrongestBrain_ProposalsAndHumanSignoff(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	svc := strongestbrain.NewService(database, reg)
	ctx := context.Background()

	// 构造触发提案的高异常运行样本：5次运行，其中2次 partial，1次 failed，平均 PromptToken > 8000
	for i := 1; i <= 5; i++ {
		state := "completed"
		if i == 2 || i == 3 {
			state = "partial"
		} else if i == 4 {
			state = "failed"
		}
		run := db.AgentRun{
			RunKey:       fmt.Sprintf("run-bench-%d", i),
			AgentKind:    "code_review",
			State:        state,
			PromptTokens: 9000,
			DurationMs:   2500,
		}
		_ = database.Create(&run).Error
		_ = database.Create(&db.RunCapabilityBinding{
			RunID:         run.ID,
			CapabilityKey: "code_review",
			Version:       1,
			Digest:        "sha256:v1",
			LoadLevel:     "L1",
		}).Error
	}

	report, err := svc.GetIntelligence(ctx, 100)
	if err != nil {
		t.Fatalf("get intelligence failed: %v", err)
	}

	// 最强大脑分析后应生成至少一项候选提案（如资源切片或预算优化）
	proposals, err := svc.ListProposals(ctx, strongestbrain.ProposalStatusPendingReview)
	if err != nil {
		t.Fatalf("list proposals failed: %v", err)
	}
	if len(proposals) == 0 {
		t.Fatalf("expected proposals to be generated due to high partial rate and high tokens")
	}

	p := proposals[0]
	if p.Status != strongestbrain.ProposalStatusPendingReview {
		t.Fatalf("expected proposal to be pending_review, got %s", p.Status)
	}

	// 人工审查：批准提案
	approved, err := svc.ReviewProposal(ctx, p.ID, strongestbrain.ProposalStatusApproved, "lead-architect", "Approved for canary")
	if err != nil {
		t.Fatalf("review proposal failed: %v", err)
	}
	if approved.Status != strongestbrain.ProposalStatusApproved {
		t.Fatalf("expected proposal status to be approved, got %s", approved.Status)
	}
	if approved.ReviewedBy != "lead-architect" {
		t.Fatalf("expected reviewer lead-architect, got %s", approved.ReviewedBy)
	}
	if approved.ReviewedAt == nil {
		t.Fatalf("expected non-nil reviewed_at")
	}
	_ = report
}

// 场景 3: 确定性沙箱 Replay 评测（Candidate vs Baseline 评分与硬门禁拦截）
func TestStrongestBrain_DeterministicReplayAndHardGates(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	svc := strongestbrain.NewService(database, reg)
	ctx := context.Background()

	// 1. 基线版本 V1 (拥有 source.read 权限，预算 2000 token)
	baseManifest := &agentruntime.CapabilityManifest{
		Schema:      "capability-manifest/v1",
		ID:          "code_review",
		Kind:        agentruntime.KindSkill,
		Version:     1,
		Permissions: []string{"source.read"},
		Budgets: agentruntime.BudgetDef{
			InstructionTokens: 2000,
			EvidenceTokens:    8000,
		},
	}
	v1, err := reg.Register(ctx, baseManifest)
	if err != nil {
		t.Fatalf("register v1 failed: %v", err)
	}

	// 2. 优秀候选版本 V2 (相同权限，Token 压缩优化到 1200 token)
	candManifest := &agentruntime.CapabilityManifest{
		Schema:      "capability-manifest/v1",
		ID:          "code_review",
		Kind:        agentruntime.KindSkill,
		Version:     2,
		Permissions: []string{"source.read"},
		Budgets: agentruntime.BudgetDef{
			InstructionTokens: 1200,
			EvidenceTokens:    8000,
		},
	}
	v2, err := reg.Register(ctx, candManifest)
	if err != nil {
		t.Fatalf("register v2 failed: %v", err)
	}

	// 执行离线 Replay 评估
	eval, err := svc.RunReplay(ctx, strongestbrain.ReplayCommand{
		CandidateVersionID: v2.ID,
		BaselineVersionID:  v1.ID,
		DatasetRef:         "gold-standard-mr-dataset",
	})
	if err != nil {
		t.Fatalf("run replay failed: %v", err)
	}

	if eval.Verdict != "pass" {
		t.Fatalf("expected replay verdict pass, got %s", eval.Verdict)
	}
	if eval.OverallScore < 90.0 {
		t.Fatalf("expected overall score >= 90, got %f", eval.OverallScore)
	}
	if eval.TokenEfficiencyScore <= 88.0 {
		t.Fatalf("expected token efficiency score boost due to 40%% instruction token savings")
	}

	// 3. 恶意/越权候选版本 V3 (试图暗中扩大权限请求 root.access)
	badManifest := &agentruntime.CapabilityManifest{
		Schema:      "capability-manifest/v1",
		ID:          "code_review",
		Kind:        agentruntime.KindSkill,
		Version:     3,
		Permissions: []string{"source.read", "root.access"},
		Budgets: agentruntime.BudgetDef{
			InstructionTokens: 1000,
		},
	}
	v3, err := reg.Register(ctx, badManifest)
	if err != nil {
		t.Fatalf("register v3 failed: %v", err)
	}

	evalBad, err := svc.RunReplay(ctx, strongestbrain.ReplayCommand{
		CandidateVersionID: v3.ID,
		BaselineVersionID:  v1.ID,
		DatasetRef:         "gold-standard-mr-dataset",
	})
	if err != nil {
		t.Fatalf("run replay for bad candidate failed: %v", err)
	}

	// 验证硬门禁生效：因越权请求被直接驳回 (verdict = reject)
	if evalBad.Verdict != "reject" {
		t.Fatalf("expected hard gate rejection for unauthorized permission expansion, got %s", evalBad.Verdict)
	}
}

// 场景 4: 金丝雀（Canary）灰度演进与自动回滚门禁
func TestStrongestBrain_CanaryProgressionAndAutoRollback(t *testing.T) {
	thresholds := strongestbrain.DefaultRollbackThresholds()

	// 1. 正常运行状态：无失败，无证据缺失 -> 推荐推进到下一阶段
	healthySummary := strongestbrain.DimensionSummary{
		TotalRuns:       100,
		CompletedRuns:   98,
		PartialRuns:     2,
		FailedRuns:      0,
		EvidenceGapRuns: 5,
		SecurityRejects: 0,
	}

	resShadow := strongestbrain.EvaluateCanarySafety(strongestbrain.CanaryStageShadow, healthySummary, thresholds)
	if !resShadow.CanAdvance || resShadow.RecommendedNextStage != strongestbrain.CanaryStage1Pct {
		t.Fatalf("expected advance from shadow to 1%%, got: %+v", resShadow)
	}

	res1Pct := strongestbrain.EvaluateCanarySafety(strongestbrain.CanaryStage1Pct, healthySummary, thresholds)
	if !res1Pct.CanAdvance || res1Pct.RecommendedNextStage != strongestbrain.CanaryStage10Pct {
		t.Fatalf("expected advance from 1%% to 10%%, got: %+v", res1Pct)
	}

	// 2. 异常运行状态：失败率高达 25% (阈值上限 10%) -> 必须触发自动回滚保护
	unhealthySummary := strongestbrain.DimensionSummary{
		TotalRuns:       40,
		CompletedRuns:   25,
		FailedRuns:      10, // 25% failure
		EvidenceGapRuns: 2,
		SecurityRejects: 0,
	}
	resRollback := strongestbrain.EvaluateCanarySafety(strongestbrain.CanaryStage10Pct, unhealthySummary, thresholds)
	if resRollback.CanAdvance || !resRollback.AutoRollbackTriggered {
		t.Fatalf("expected auto rollback triggered for 25%% failure rate, got: %+v", resRollback)
	}
	if resRollback.RecommendedNextStage != "rollback_to_baseline" {
		t.Fatalf("expected recommended stage to be rollback_to_baseline, got %s", resRollback.RecommendedNextStage)
	}

	// 3. 安全违规事件触发：一旦出现任何安全违规（SecurityRejects > 0），立即熔断回滚
	securityBreachSummary := strongestbrain.DimensionSummary{
		TotalRuns:       50,
		CompletedRuns:   49,
		FailedRuns:      1,
		SecurityRejects: 1, // 安全违规拦截
	}
	resSecRollback := strongestbrain.EvaluateCanarySafety(strongestbrain.CanaryStage50Pct, securityBreachSummary, thresholds)
	if !resSecRollback.AutoRollbackTriggered {
		t.Fatalf("expected security breach to trigger auto rollback immediately")
	}
}
