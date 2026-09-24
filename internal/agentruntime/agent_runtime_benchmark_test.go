package agentruntime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"well-ambient/internal/agentruntime"
)

// BenchmarkCapabilityRegistry_Resolve measures DAG dependency resolution throughput.
func BenchmarkCapabilityRegistry_Resolve(b *testing.B) {
	database := setupTestDB(&testing.T{})
	reg := agentruntime.NewRegistry(database)
	ctx := context.Background()

	// 注册 5 个互相依赖的能力
	for i := 1; i <= 5; i++ {
		manifest := &agentruntime.CapabilityManifest{
			Schema:  "capability-manifest/v1",
			ID:      fmt.Sprintf("cap-%d", i),
			Kind:    agentruntime.KindPlugin,
			Version: 1,
			Permissions: []string{"read"},
			Budgets: agentruntime.BudgetDef{
				InstructionTokens: 500,
				EvidenceTokens:    2000,
			},
		}
		manifest.Triggers.Intents = []string{"benchmark"}
		if i > 1 {
			manifest.Requires = []agentruntime.DependencyRef{
				{ID: fmt.Sprintf("cap-%d", i-1), Kind: agentruntime.KindPlugin, Version: ">=1"},
			}
		}
		_, _ = reg.Register(ctx, manifest)
		_ = reg.ActivateVersion(ctx, fmt.Sprintf("cap-%d", i), 1, "global", "")
	}

	query := agentruntime.ResolveQuery{
		Intent:             "benchmark",
		AllowedPermissions: []string{"read"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.Resolve(ctx, query)
		if err != nil {
			b.Fatalf("resolve failed: %v", err)
		}
	}
}

// BenchmarkContextProtocol_RenderCompactVsJSON compares token savings and rendering latency between ACP/1 and Canonical JSON.
func BenchmarkContextProtocol_RenderCompactVsJSON(b *testing.B) {
	graph := agentruntime.ContextGraph{
		RunID:             "R82",
		AgentKind:         "code_review",
		Goal:              "review-change",
		ScopeRepo:         "10",
		ScopeMR:           "42",
		ScopeHeadSHA:      "a91c2e",
		BoundCapabilities: []string{"merge-review@3", "gitlab.snapshot@2", "knowledge.search@1"},
		InstructionBudget: 1800,
		EvidenceBudget:    12000,
		RawBudget:         0,
		Facts: []agentruntime.ContextFact{
			{ID: "F1", Type: "rule", Statement: "empty feedback != success", SourceRef: "K12"},
			{ID: "F2", Type: "rule", Statement: "retry attempts must be idempotent and bounded", SourceRef: "K15"},
			{ID: "F3", Type: "policy", Statement: "strict isolation between tenants", SourceRef: "K20"},
		},
		Changes: []agentruntime.ContextChange{
			{ID: "C1", Path: "internal/dispatch/completion.go", Lines: "38:51", Status: "modified"},
			{ID: "C2", Path: "internal/server/notification_handlers.go", Lines: "120:185", Status: "modified"},
		},
		Evidence: []agentruntime.ContextEvidence{
			{ID: "E1", TargetChange: "C1:44", Snippet: "return feedback == \"\""},
			{ID: "E2", TargetChange: "C2:145", Snippet: "if status == StatusFailed { retryCount++ }"},
		},
		OpenQuestions: []agentruntime.ContextOpenQuestion{
			{ID: "Q1", Question: "vehicle-service completion contract missing"},
		},
		Refs: []agentruntime.ContextRef{
			{ID: "K12", Target: "knowledge:182@v3"},
			{ID: "K15", Target: "knowledge:204@v1"},
			{ID: "K20", Target: "policy:tenant-isolation@v2"},
		},
	}

	// Calculate and verify token reduction
	compactView := graph.RenderCompactView()
	jsonBytes, _ := json.MarshalIndent(graph, "", "  ")

	compactTokens := agentruntime.EstimateTokens(compactView)
	jsonTokens := agentruntime.EstimateTokens(string(jsonBytes))

	savingsPct := float64(jsonTokens-compactTokens) / float64(jsonTokens) * 100

	b.Logf("Tokens comparison: Canonical JSON = %d tokens, ACP/1 Compact = %d tokens, Savings = %.2f%%",
		jsonTokens, compactTokens, savingsPct)

	if savingsPct < 30.0 {
		b.Fatalf("ACP/1 token savings %.2f%% is below the required 30%% target", savingsPct)
	}

	b.Run("ACP1_RenderCompact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = graph.RenderCompactView()
		}
	})

	b.Run("Canonical_JSON", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(graph)
		}
	})
}

// BenchmarkRunLockfile_ComputeAndVerifyHash measures the throughput of hash generation and verification.
func BenchmarkRunLockfile_ComputeAndVerifyHash(b *testing.B) {
	lockfile := agentruntime.RunLockfile{
		Schema:    "agent-run-lockfile/v1",
		RunID:     "run-bench-101",
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

	hash, _ := lockfile.ComputeHash()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !lockfile.VerifyHash(hash) {
			b.Fatal("verify hash failed")
		}
	}
}
