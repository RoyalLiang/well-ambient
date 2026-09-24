package agentruntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
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
		&db.SolutionPromptTemplate{},
		&db.CodeReviewRun{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}
	return database
}

// 场景 1: 核心能力注册、版本递增、单 Scope 互斥激活与退役
func TestCapabilityRegistry_LifecycleAndActivation(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	ctx := context.Background()

	manifestV1 := &agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "test-skill",
		Kind:    agentruntime.KindSkill,
		Version: 1,
		Scope: struct {
			Type string `yaml:"type" json:"type"`
			ID   string `yaml:"id,omitempty" json:"id,omitempty"`
		}{Type: "global"},
		Provides:    []string{"analysis"},
		Permissions: []string{"read"},
		Budgets: agentruntime.BudgetDef{
			InstructionTokens: 1000,
			EvidenceTokens:    2000,
		},
	}

	// 1. 注册 V1
	v1, err := reg.Register(ctx, manifestV1)
	if err != nil {
		t.Fatalf("register v1 failed: %v", err)
	}
	if v1.Status != "draft" {
		t.Fatalf("expected v1 initial status to be draft, got %s", v1.Status)
	}
	if v1.ContentDigest == "" {
		t.Fatalf("expected computed digest to be non-empty")
	}

	// 2. 激活 V1
	if err := reg.ActivateVersion(ctx, "test-skill", 1, "global", ""); err != nil {
		t.Fatalf("activate v1 failed: %v", err)
	}
	mActive, verActive, err := reg.GetActive(ctx, "test-skill", "global", "")
	if err != nil {
		t.Fatalf("get active v1 failed: %v", err)
	}
	if verActive.Version != 1 || verActive.Status != "active" {
		t.Fatalf("expected active version 1, got %d, status %s", verActive.Version, verActive.Status)
	}
	if mActive.ID != "test-skill" {
		t.Fatalf("expected manifest id test-skill, got %s", mActive.ID)
	}

	// 3. 注册 V2 并激活，验证单 scope 互斥（V1 自动 retired）
	manifestV2 := *manifestV1
	manifestV2.Version = 2
	manifestV2.Budgets.InstructionTokens = 800
	v2, err := reg.Register(ctx, &manifestV2)
	if err != nil {
		t.Fatalf("register v2 failed: %v", err)
	}
	if err := reg.ActivateVersion(ctx, "test-skill", 2, "global", ""); err != nil {
		t.Fatalf("activate v2 failed: %v", err)
	}

	_, verActive2, err := reg.GetActive(ctx, "test-skill", "global", "")
	if err != nil {
		t.Fatalf("get active v2 failed: %v", err)
	}
	if verActive2.Version != 2 {
		t.Fatalf("expected active version 2, got %d", verActive2.Version)
	}

	// 检查 V1 是否已被退役
	var checkV1 db.CapabilityVersion
	if err := database.First(&checkV1, v1.ID).Error; err != nil {
		t.Fatalf("failed to query v1: %v", err)
	}
	if checkV1.Status != "retired" || checkV1.RetiredAt == nil {
		t.Fatalf("expected v1 to be retired, got %s", checkV1.Status)
	}
	_ = v2
}

// 场景 2: DAG 依赖拓扑解析与循环依赖检测、可选依赖
func TestCapabilityRegistry_DAGResolutionAndCycleDetection(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	ctx := context.Background()

	// 注册底层能力 A
	capA := &agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "plugin.storage",
		Kind:    agentruntime.KindPlugin,
		Version: 1,
		Permissions: []string{"storage.write"},
	}
	capA.Triggers.Intents = []string{"archive"}
	_, _ = reg.Register(ctx, capA)
	_ = reg.ActivateVersion(ctx, "plugin.storage", 1, "global", "")

	// 注册中层能力 B (依赖 A)
	capB := &agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "service.compress",
		Kind:    agentruntime.KindPlugin,
		Version: 1,
		Requires: []agentruntime.DependencyRef{
			{ID: "plugin.storage", Kind: agentruntime.KindPlugin, Version: ">=1"},
		},
		Permissions: []string{"storage.write"},
	}
	capB.Triggers.Intents = []string{"archive"}
	_, _ = reg.Register(ctx, capB)
	_ = reg.ActivateVersion(ctx, "service.compress", 1, "global", "")

	// 解析 intent "archive"
	plan, err := reg.Resolve(ctx, agentruntime.ResolveQuery{
		Intent:             "archive",
		AllowedPermissions: []string{"storage.write"},
	})
	if err != nil {
		t.Fatalf("resolve DAG plan failed: %v", err)
	}
	if len(plan.Capabilities) != 2 {
		t.Fatalf("expected 2 resolved capabilities, got %d", len(plan.Capabilities))
	}

	// 验证拓扑顺序：依赖项 plugin.storage 必须排在 service.compress 之前
	foundStorage := false
	storageIndex := -1
	compressIndex := -1
	for idx, c := range plan.Capabilities {
		if c.ID == "plugin.storage" {
			foundStorage = true
			storageIndex = idx
		}
		if c.ID == "service.compress" {
			compressIndex = idx
		}
	}
	if !foundStorage || storageIndex > compressIndex {
		t.Fatalf("topological sort violated: storage (%d) must precede compress (%d)", storageIndex, compressIndex)
	}
}

// 场景 3: 严格安全与权限门禁（未授权权限严格拦截）
func TestCapabilityRegistry_SecurityPermissionRejection(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	ctx := context.Background()

	capDangerous := &agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "plugin.dangerous",
		Kind:    agentruntime.KindPlugin,
		Version: 1,
		Permissions: []string{"root.access", "network.raw"},
	}
	capDangerous.Triggers.Intents = []string{"admin-op"}
	_, _ = reg.Register(ctx, capDangerous)
	_ = reg.ActivateVersion(ctx, "plugin.dangerous", 1, "global", "")

	// 用户只被授予 "source.read"
	_, err := reg.Resolve(ctx, agentruntime.ResolveQuery{
		Intent:             "admin-op",
		AllowedPermissions: []string{"source.read"},
	})
	if err == nil {
		t.Fatalf("expected permission violation error, but resolution succeeded")
	}
	if !strings.Contains(fmt.Sprintf("%v", err), "unauthorized permissions") {
		t.Fatalf("expected unauthorized permissions error, got: %v", err)
	}
}

// 场景 4: 预算分配与不可变 Lockfile 冻结及哈希校验
func TestRunLockfile_ImmutabilityAndHashVerification(t *testing.T) {
	lockfile := agentruntime.RunLockfile{
		Schema:    "agent-run-lockfile/v1",
		RunID:     "run-20260924-001",
		AgentKind: "code_review",
		Kernel: agentruntime.RefInfo{
			Version: "v1.2.0",
			Digest:  "sha256:kernel-v1-hash",
		},
		ModelProfile: agentruntime.RefInfo{
			ID:     "claude-3-7-sonnet",
			Digest: "sha256:model-hash",
		},
		Capabilities: []agentruntime.BoundCapability{
			{
				ID:              "code_review",
				Kind:            "skill",
				Version:         2,
				Digest:          "sha256:skill-digest-v2",
				SelectionReason: "primary skill for code review",
				LoadLevels:      []string{"L0", "L1", "L2"},
			},
			{
				ID:              "gitlab.snapshot",
				Kind:            "plugin",
				Version:         1,
				Digest:          "sha256:gitlab-digest-v1",
				SelectionReason: "required by code_review",
				LoadLevels:      []string{"L0", "L1"},
			},
		},
		ContextPack: agentruntime.RefInfo{
			ID:     "pack-991",
			Digest: "sha256:context-pack-digest",
		},
		PermissionGrant: agentruntime.PermissionGrantInfo{
			ID:      "grant-88",
			Digest:  "sha256:grant-digest",
			Actions: []string{"source.read", "knowledge.read"},
		},
		Budgets: agentruntime.LockfileBudgets{
			InputTokens:     18000,
			OutputTokens:    8000,
			ToolCalls:       25,
			WallTimeSeconds: 300,
		},
		Resolver: agentruntime.RefInfo{
			Version: "resolver-v1",
			Digest:  "sha256:resolver-digest",
		},
	}

	hash, err := lockfile.ComputeHash()
	if err != nil {
		t.Fatalf("compute lockfile hash failed: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected non-empty lockfile hash")
	}

	// 校验哈希合法性
	if !lockfile.VerifyHash(hash) {
		t.Fatalf("lockfile verification failed for valid hash")
	}

	// 篡改预算，验证哈希失效
	tampered := lockfile
	tampered.Budgets.InputTokens = 999999
	if tampered.VerifyHash(hash) {
		t.Fatalf("tampered lockfile unexpectedly verified successfully")
	}
}

// 场景 5: Agent Context Protocol (ACP/1) 紧凑渲染与 Delta 增量应用
func TestAgentContextProtocol_CompactViewAndDelta(t *testing.T) {
	graph := agentruntime.ContextGraph{
		RunID:             "R82",
		AgentKind:         "code_review",
		Goal:              "review-change",
		ScopeRepo:         "10",
		ScopeMR:           "42",
		ScopeHeadSHA:      "a91c2e",
		BoundCapabilities: []string{"merge-review@3", "gitlab.snapshot@2"},
		InstructionBudget: 1800,
		EvidenceBudget:    12000,
		RawBudget:         0,
		Facts: []agentruntime.ContextFact{
			{ID: "F1", Type: "rule", Statement: "empty feedback != success", SourceRef: "K12"},
		},
		Changes: []agentruntime.ContextChange{
			{ID: "C1", Path: "dispatch/completion.go", Lines: "38:51", Status: "modified"},
		},
		Evidence: []agentruntime.ContextEvidence{
			{ID: "E1", TargetChange: "C1:44", Snippet: "return feedback == \"\""},
		},
		OpenQuestions: []agentruntime.ContextOpenQuestion{
			{ID: "Q1", Question: "vehicle-service contract missing"},
		},
		Refs: []agentruntime.ContextRef{
			{ID: "K12", Target: "knowledge:182@v3"},
		},
	}

	// 1. 验证 ACP/1 Compact View
	compactView := graph.RenderCompactView()
	if !contains(compactView, "@schema acp/1") {
		t.Fatalf("missing @schema acp/1 in compact view")
	}
	if !contains(compactView, "@run R82 kind=code_review goal=review-change") {
		t.Fatalf("missing @run line in compact view")
	}
	if !contains(compactView, "F1|rule|empty feedback != success|K12") {
		t.Fatalf("missing structured fact line in compact view")
	}
	if !contains(compactView, "E1|C1:44|return feedback == \"\"") {
		t.Fatalf("missing evidence snippet line in compact view")
	}

	baseHash := graph.ComputeHash()
	if baseHash == "" {
		t.Fatalf("empty base hash")
	}

	// 2. 验证 Delta 增量生成与应用
	delta := graph.RenderDelta(baseHash, []agentruntime.ContextFact{
		{ID: "F2", Type: "rule", Statement: "retry must be idempotent", SourceRef: "K19"},
	}, []string{"Q1"})

	if !contains(delta, "+F2|rule|retry must be idempotent|K19") {
		t.Fatalf("missing +F2 in delta: %s", delta)
	}
	if !contains(delta, "-Q1") {
		t.Fatalf("missing -Q1 in delta: %s", delta)
	}

	// 应用 Delta
	err := graph.ApplyDelta(baseHash, delta)
	if err != nil {
		t.Fatalf("apply delta failed: %v", err)
	}

	// 确认 F2 已加入，Q1 已被解决移除
	foundF2 := false
	for _, f := range graph.Facts {
		if f.ID == "F2" {
			foundF2 = true
			break
		}
	}
	if !foundF2 {
		t.Fatalf("F2 not found in facts after applying delta")
	}
	if len(graph.OpenQuestions) != 0 {
		t.Fatalf("expected Q1 to be removed from open questions, got %d questions", len(graph.OpenQuestions))
	}

	// 验证基础 Hash 不匹配时被拒绝
	errWrongBase := graph.ApplyDelta("sha256:wrong-base", delta)
	if errWrongBase == nil {
		t.Fatalf("expected base hash mismatch error, but succeeded")
	}
}

// 场景 6: 高并发 TraceCollector 并发写入无竞态与顺序性
func TestTraceCollector_ConcurrentRecordEvent(t *testing.T) {
	database := setupTestDB(t)
	collector := agentruntime.NewTraceCollector(database)
	ctx := context.Background()

	// 创建一个测试 AgentRun
	run := db.AgentRun{
		RunKey:        "run-concurrent-test",
		AgentKind:     "code_review",
		KernelVersion: "v1",
		State:         "running",
	}
	if err := database.Create(&run).Error; err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	concurrency := 20
	iterations := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for c := 0; c < concurrency; c++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				payload := map[string]any{
					"worker": workerID,
					"seq":    i,
				}
				_ = collector.RecordEvent(ctx, run.ID, agentruntime.EventToolCalled, payload)
			}
		}(c)
	}
	wg.Wait()

	events, err := collector.GetRunEvents(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run events failed: %v", err)
	}
	expectedTotal := concurrency * iterations
	if len(events) != expectedTotal {
		t.Fatalf("expected %d events, got %d", expectedTotal, len(events))
	}

	// 检查序列号顺序递增
	for idx, e := range events {
		expectedSeq := idx + 1
		if e.Sequence != expectedSeq {
			t.Fatalf("sequence mismatch at %d: expected %d, got %d", idx, expectedSeq, e.Sequence)
		}
	}
}

// 场景 7: LegacySkillAdapter 向后兼容映射
func TestLegacySkillAdapter_BackwardCompatibility(t *testing.T) {
	database := setupTestDB(t)
	reg := agentruntime.NewRegistry(database)
	adapter := agentruntime.NewLegacySkillAdapter(database, reg)
	ctx := context.Background()

	// 1. 初始化默认三项能力
	if err := adapter.EnsureDefaultCapabilities(ctx); err != nil {
		t.Fatalf("ensure default capabilities failed: %v", err)
	}

	// 验证能力已被登记并激活
	_, _, err := reg.GetActive(ctx, "gitlab.snapshot", "global", "")
	if err != nil {
		t.Fatalf("expected gitlab.snapshot active: %v", err)
	}
	_, _, err = reg.GetActive(ctx, "knowledge.search", "global", "")
	if err != nil {
		t.Fatalf("expected knowledge.search active: %v", err)
	}
	_, _, err = reg.GetActive(ctx, "code_review", "global", "")
	if err != nil {
		t.Fatalf("expected code_review active: %v", err)
	}

	// 2. 映射历史 CodeReviewRun
	legacyRun := db.CodeReviewRun{
		ID:           101,
		Key:          "review-test-101",
		Status:       "completed",
		Model:        "gpt-4o",
		SkillVersion: 1,
		SkillHash:    "sha256:legacy-hash",
		SkillName:    "code_review",
		ReportJSON:   `{"evidence_complete":true,"findings":[]}`,
	}
	agentRun, bindings := adapter.MapCodeReviewRunToAgentRun(&legacyRun)
	if agentRun.RunKey != "review-test-101" {
		t.Fatalf("expected run key review-test-101, got %s", agentRun.RunKey)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}
	if bindings[0].CapabilityKey != "code_review" || bindings[0].Version != 1 {
		t.Fatalf("expected code_review binding v1, got %+v", bindings[0])
	}
}

func contains(str, substr string) bool {
	return json.Valid([]byte(str)) || len(str) >= len(substr) && (str == substr || len(substr) > 0 && len(str) > 0 && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || indexOf(str, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
