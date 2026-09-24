package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/strongestbrain"
)

func TestStrongestBrainCapabilityHandlers_FullE2EScenario(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	authHeader := "Bearer " + superAdminToken(t, "superadmin@example.com", "Super Admin", nil)

	// 1. 测试 GET /api/agent-runtime/capabilities (初始应已 seed 默认三项能力)
	req := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/capabilities", nil)
	req.Header.Set("Authorization", authHeader)
	w := httptest.NewRecorder()
	server.handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for list capabilities, got %d, body: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Capabilities []struct {
			CapabilityKey string `json:"capability_key"`
		} `json:"capabilities"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list capabilities failed: %v", err)
	}
	if listResp.Total < 3 {
		t.Fatalf("expected at least 3 seeded capabilities, got %d", listResp.Total)
	}

	// 2. 测试 POST /api/agent-runtime/capabilities 注册新能力
	newSkillManifest := agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "security.scanner",
		Kind:    agentruntime.KindPlugin,
		Version: 1,
		Permissions: []string{"source.read"},
		Budgets: agentruntime.BudgetDef{
			InstructionTokens: 800,
			EvidenceTokens:    3000,
		},
	}
	newSkillManifest.Scope.Type = "global"
	newSkillManifest.Triggers.Intents = []string{"sec-audit"}
	bodyBytes, _ := json.Marshal(newSkillManifest)

	reqReg := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities", bytes.NewReader(bodyBytes))
	reqReg.Header.Set("Authorization", authHeader)
	reqReg.Header.Set("Content-Type", "application/json")
	wReg := httptest.NewRecorder()
	server.handler.ServeHTTP(wReg, reqReg)
	if wReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 for register capability, got %d, body: %s", wReg.Code, wReg.Body.String())
	}

	// 3. 测试 POST /api/agent-runtime/capabilities/{id}/activate 激活新能力
	activateBody, _ := json.Marshal(map[string]any{
		"version":    1,
		"scope_type": "global",
	})
	reqAct := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities/security.scanner/activate", bytes.NewReader(activateBody))
	reqAct.Header.Set("Authorization", authHeader)
	reqAct.Header.Set("Content-Type", "application/json")
	wAct := httptest.NewRecorder()
	server.handler.ServeHTTP(wAct, reqAct)
	if wAct.Code != http.StatusOK {
		t.Fatalf("expected 200 for activate capability, got %d, body: %s", wAct.Code, wAct.Body.String())
	}

	// 4. 测试 POST /api/agent-runtime/resolve/preview 能力依赖拓扑解析预览
	previewBody, _ := json.Marshal(agentruntime.ResolveQuery{
		Intent:             "sec-audit",
		AllowedPermissions: []string{"source.read"},
	})
	reqPrev := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/resolve/preview", bytes.NewReader(previewBody))
	reqPrev.Header.Set("Authorization", authHeader)
	reqPrev.Header.Set("Content-Type", "application/json")
	wPrev := httptest.NewRecorder()
	server.handler.ServeHTTP(wPrev, reqPrev)
	if wPrev.Code != http.StatusOK {
		t.Fatalf("expected 200 for preview resolve, got %d, body: %s", wPrev.Code, wPrev.Body.String())
	}
	var plan agentruntime.CapabilityPlan
	if err := json.NewDecoder(wPrev.Body).Decode(&plan); err != nil {
		t.Fatalf("decode resolve plan failed: %v", err)
	}
	if len(plan.Capabilities) != 1 || plan.Capabilities[0].ID != "security.scanner" {
		t.Fatalf("expected resolved security.scanner capability, got: %+v", plan.Capabilities)
	}

	// 5. 创建一条 AgentRun 与绑定，测试 Lockfile 与 Trace 接口
	testRun := db.AgentRun{
		RunKey:          "run-api-test-01",
		AgentKind:       "code_review",
		KernelVersion:   "v1",
		ModelProfileRef: "claude-sonnet",
		State:           "completed",
		PromptTokens:    3200,
		DurationMs:      1500,
	}
	if err := db.DB.Create(&testRun).Error; err != nil {
		t.Fatalf("create test run failed: %v", err)
	}
	_ = db.DB.Create(&db.RunCapabilityBinding{
		RunID:         testRun.ID,
		CapabilityKey: "code_review",
		Version:       1,
		Digest:        "sha256:digest-code-review-v1",
		LoadLevel:     "L1",
	}).Error

	// 写入 Trace 事件
	_ = server.agentTrace.RecordEvent(req.Context(), testRun.ID, agentruntime.EventIntentCompiled, map[string]any{"intent": "code-review"})
	_ = server.agentTrace.RecordEvent(req.Context(), testRun.ID, agentruntime.EventCompleted, map[string]any{"result": "pass"})

	// GET /api/agent-runtime/runs
	reqRuns := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/runs", nil)
	reqRuns.Header.Set("Authorization", authHeader)
	wRuns := httptest.NewRecorder()
	server.handler.ServeHTTP(wRuns, reqRuns)
	if wRuns.Code != http.StatusOK {
		t.Fatalf("expected 200 for list runs, got %d", wRuns.Code)
	}

	// GET /api/agent-runtime/runs/{id}/lockfile
	reqLock := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/runs/1/lockfile", nil)
	reqLock.SetPathValue("id", "1")
	reqLock.Header.Set("Authorization", authHeader)
	wLock := httptest.NewRecorder()
	server.handler.ServeHTTP(wLock, reqLock)
	if wLock.Code != http.StatusOK {
		t.Fatalf("expected 200 for get lockfile, got %d, body: %s", wLock.Code, wLock.Body.String())
	}
	if wLock.Header().Get("x-lockfile-hash") == "" {
		t.Fatalf("expected x-lockfile-hash header to be present")
	}

	// GET /api/agent-runtime/runs/{id}/trace
	reqTrace := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/runs/1/trace", nil)
	reqTrace.SetPathValue("id", "1")
	reqTrace.Header.Set("Authorization", authHeader)
	wTrace := httptest.NewRecorder()
	server.handler.ServeHTTP(wTrace, reqTrace)
	if wTrace.Code != http.StatusOK {
		t.Fatalf("expected 200 for get trace, got %d, body: %s", wTrace.Code, wTrace.Body.String())
	}

	// 6. 测试 GET /api/strongest-brain/capability-intelligence
	reqIntel := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/capability-intelligence", nil)
	reqIntel.Header.Set("Authorization", authHeader)
	wIntel := httptest.NewRecorder()
	server.handler.ServeHTTP(wIntel, reqIntel)
	if wIntel.Code != http.StatusOK {
		t.Fatalf("expected 200 for capability intelligence, got %d, body: %s", wIntel.Code, wIntel.Body.String())
	}
	var intelResp strongestbrain.CapabilityIntelligenceReport
	if err := json.NewDecoder(wIntel.Body).Decode(&intelResp); err != nil {
		t.Fatalf("decode intelligence report failed: %v", err)
	}
	if intelResp.Summary.TotalRuns < 1 {
		t.Fatalf("expected total runs >= 1 in intelligence report, got %d", intelResp.Summary.TotalRuns)
	}

	// 7. 测试 Proposal 生成与审核 API
	// 先生成一条 Proposal
	proposal := db.CapabilityProposal{
		Title:               "优化代码审查 Token 预算",
		ProposalType:        strongestbrain.ProposalTypeBudgetRealloc,
		Status:              strongestbrain.ProposalStatusPendingReview,
		TargetCapabilityKey: "code_review",
		ChangesJSON:         `{"budget":1000}`,
	}
	if err := db.DB.Create(&proposal).Error; err != nil {
		t.Fatalf("create proposal failed: %v", err)
	}

	// GET /api/strongest-brain/capability-proposals
	reqProps := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/capability-proposals", nil)
	reqProps.Header.Set("Authorization", authHeader)
	wProps := httptest.NewRecorder()
	server.handler.ServeHTTP(wProps, reqProps)
	if wProps.Code != http.StatusOK {
		t.Fatalf("expected 200 for list proposals, got %d", wProps.Code)
	}

	// POST /api/strongest-brain/capability-proposals/{id}/review 审核提案
	reviewBody, _ := json.Marshal(map[string]any{
		"decision": "approved",
		"notes":    "E2E verified in testing harness",
	})
	reqRev := httptest.NewRequest(http.MethodPost, "/api/strongest-brain/capability-proposals/1/review", bytes.NewReader(reviewBody))
	reqRev.SetPathValue("id", "1")
	reqRev.Header.Set("Authorization", authHeader)
	reqRev.Header.Set("Content-Type", "application/json")
	wRev := httptest.NewRecorder()
	server.handler.ServeHTTP(wRev, reqRev)
	if wRev.Code != http.StatusOK {
		t.Fatalf("expected 200 for review proposal, got %d, body: %s", wRev.Code, wRev.Body.String())
	}
}
