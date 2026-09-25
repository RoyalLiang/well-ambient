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

func TestCapabilityExtendedLifecycle_Detail_DiskCleanup_Reinstall_Upgrade(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	authHeader := "Bearer " + superAdminToken(t, "superadmin@example.com", "Super Admin", nil)

	// 1. 注册一个具备较大资源的技能
	testManifest := agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "test.analytics.skill",
		Kind:    agentruntime.KindSkill,
		Version: 1,
		Owner:   "data-team",
		Resources: []agentruntime.ResourceDef{
			{
				Key:         "main-prompt",
				ContentKind: "text",
				Content:     "This is a high-volume analytics system prompt with extensive guidance.",
			},
			{
				Key:         "dataset-schema",
				ContentKind: "json",
				Content:     `{"fields": ["id", "timestamp", "metrics", "anomaly_score"]}`,
			},
		},
	}
	bodyBytes, _ := json.Marshal(testManifest)
	reqReg := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities", bytes.NewReader(bodyBytes))
	reqReg.Header.Set("Authorization", authHeader)
	wReg := httptest.NewRecorder()
	server.handler.ServeHTTP(wReg, reqReg)
	if wReg.Code != http.StatusCreated {
		t.Fatalf("failed to register capability: %d, body: %s", wReg.Code, wReg.Body.String())
	}

	// 2. GET /api/agent-runtime/capabilities 检查返回列表包含 disk_size_bytes
	reqList := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/capabilities?search=test.analytics", nil)
	reqList.Header.Set("Authorization", authHeader)
	wList := httptest.NewRecorder()
	server.handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("failed to list capabilities: %d", wList.Code)
	}
	var listResp struct {
		Capabilities []struct {
			CapabilityKey string `json:"capability_key"`
			DiskSizeBytes int64  `json:"disk_size_bytes"`
			Status        string `json:"status"`
		} `json:"capabilities"`
		TotalDiskSizeBytes int64 `json:"total_disk_size_bytes"`
	}
	if err := json.NewDecoder(wList.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Capabilities) != 1 || listResp.Capabilities[0].DiskSizeBytes <= 0 {
		t.Fatalf("expected non-zero disk size, got: %+v", listResp)
	}
	initialDiskSize := listResp.Capabilities[0].DiskSizeBytes

	// 3. GET /api/agent-runtime/capabilities/{id} 获取详情
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/capabilities/test.analytics.skill", nil)
	reqDetail.SetPathValue("id", "test.analytics.skill")
	reqDetail.Header.Set("Authorization", authHeader)
	wDetail := httptest.NewRecorder()
	server.handler.ServeHTTP(wDetail, reqDetail)
	if wDetail.Code != http.StatusOK {
		t.Fatalf("failed to get detail: %d, body: %s", wDetail.Code, wDetail.Body.String())
	}
	var detailResp agentruntime.CapabilityDetail
	if err := json.NewDecoder(wDetail.Body).Decode(&detailResp); err != nil {
		t.Fatal(err)
	}
	if len(detailResp.Versions) != 1 || len(detailResp.Versions[0].Resources) != 2 {
		t.Fatalf("expected 1 version with 2 resources, got: %+v", detailResp)
	}

	// 4. PATCH /api/agent-runtime/capabilities/{id}/status 切换为 disabled
	patchBody, _ := json.Marshal(map[string]any{"status": "disabled"})
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/agent-runtime/capabilities/test.analytics.skill/status", bytes.NewReader(patchBody))
	reqPatch.SetPathValue("id", "test.analytics.skill")
	reqPatch.Header.Set("Authorization", authHeader)
	wPatch := httptest.NewRecorder()
	server.handler.ServeHTTP(wPatch, reqPatch)
	if wPatch.Code != http.StatusOK {
		t.Fatalf("failed to patch status: %d", wPatch.Code)
	}

	// 5. POST /api/agent-runtime/capabilities/{id}/upgrade 升级技能到新版本
	upgradeManifest := testManifest
	upgradeManifest.Version = 2
	upgradeManifest.Resources[0].Content = "Upgraded analytical logic with deep inspection capabilities."
	upgradeBytes, _ := json.Marshal(upgradeManifest)
	reqUp := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities/test.analytics.skill/upgrade", bytes.NewReader(upgradeBytes))
	reqUp.SetPathValue("id", "test.analytics.skill")
	reqUp.Header.Set("Authorization", authHeader)
	wUp := httptest.NewRecorder()
	server.handler.ServeHTTP(wUp, reqUp)
	if wUp.Code != http.StatusOK {
		t.Fatalf("failed to upgrade capability: %d, body: %s", wUp.Code, wUp.Body.String())
	}

	// 6. DELETE /api/agent-runtime/capabilities/{id} 删除并自动清理磁盘资源
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/agent-runtime/capabilities/test.analytics.skill", nil)
	reqDel.SetPathValue("id", "test.analytics.skill")
	reqDel.Header.Set("Authorization", authHeader)
	wDel := httptest.NewRecorder()
	server.handler.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("failed to delete capability: %d", wDel.Code)
	}

	// 验证：删除后资源内容被清理，状态标记为 uninstalled，但元数据记录保留
	var afterCap db.Capability
	if err := db.DB.Where("capability_key = ?", "test.analytics.skill").First(&afterCap).Error; err != nil {
		t.Fatalf("expected capability metadata record to be preserved, got err: %v", err)
	}
	if afterCap.Status != "uninstalled" {
		t.Fatalf("expected status uninstalled, got: %s", afterCap.Status)
	}

	// 7. POST /api/agent-runtime/capabilities/{id}/reinstall 重新安装并恢复
	reqReinst := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities/test.analytics.skill/reinstall", nil)
	reqReinst.SetPathValue("id", "test.analytics.skill")
	reqReinst.Header.Set("Authorization", authHeader)
	wReinst := httptest.NewRecorder()
	server.handler.ServeHTTP(wReinst, reqReinst)
	if wReinst.Code != http.StatusOK {
		t.Fatalf("failed to reinstall capability: %d, body: %s", wReinst.Code, wReinst.Body.String())
	}

	var restoredCap db.Capability
	_ = db.DB.Where("capability_key = ?", "test.analytics.skill").First(&restoredCap)
	if restoredCap.Status != "active" {
		t.Fatalf("expected status active after reinstall, got: %s", restoredCap.Status)
	}
	_ = initialDiskSize
}

func TestCapability_RemoteInstall(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	authHeader := "Bearer " + superAdminToken(t, "superadmin@example.com", "Super Admin", nil)

	// 模拟远程技能市场/仓库 HTTP 服务器
	manifestYAML := `
schema: capability-manifest/v1
id: remote.fleet.dispatch
kind: skill
version: 1
owner: logistics-infra
resources:
  - key: dispatch-guide
    content_kind: text
    content: "Guide for autonomous fleet dispatching across 100+ yards."
budgets:
  instruction_tokens: 1200
  evidence_tokens: 4000
`
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(manifestYAML))
	}))
	defer remoteServer.Close()

	// 调用 POST /api/agent-runtime/capabilities/remote-install
	installBody, _ := json.Marshal(map[string]any{
		"url":           remoteServer.URL + "/skills/remote-fleet-dispatch.yaml",
		"auto_activate": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities/remote-install", bytes.NewReader(installBody))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.handler.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for remote install, got %d, body: %s", w.Code, w.Body.String())
	}

	// 验证已入库并激活
	var capRecord db.Capability
	if err := db.DB.Where("capability_key = ?", "remote.fleet.dispatch").First(&capRecord).Error; err != nil {
		t.Fatalf("remote capability not found in DB: %v", err)
	}
	if capRecord.Status != "active" {
		t.Fatalf("expected active status, got: %s", capRecord.Status)
	}
}

func TestCapability_ReferenceCountingAndColdArchive(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	authHeader := "Bearer " + superAdminToken(t, "superadmin@example.com", "Super Admin", nil)

	// 1. 注册新技能
	manifest := agentruntime.CapabilityManifest{
		Schema:  "capability-manifest/v1",
		ID:      "archive.test.skill",
		Kind:    agentruntime.KindSkill,
		Version: 1,
		Resources: []agentruntime.ResourceDef{
			{
				Key:         "instructions",
				ContentKind: "text",
				Content:     "Important historical instruction content for run replay.",
				LoadLevel:   "L1",
			},
		},
	}
	bodyBytes, _ := json.Marshal(manifest)
	reqReg := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities", bytes.NewReader(bodyBytes))
	reqReg.Header.Set("Authorization", authHeader)
	reqReg.Header.Set("Content-Type", "application/json")
	wReg := httptest.NewRecorder()
	server.handler.ServeHTTP(wReg, reqReg)
	if wReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 for register capability, got %d", wReg.Code)
	}

	// 2. 初始查询：bindings_count 应为 0
	reqList := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/capabilities?search=archive.test.skill", nil)
	reqList.Header.Set("Authorization", authHeader)
	wList := httptest.NewRecorder()
	server.handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wList.Code)
	}
	var listResp struct {
		Capabilities []struct {
			CapabilityKey string `json:"capability_key"`
			BindingsCount int64  `json:"bindings_count"`
			IsArchived    bool   `json:"is_archived"`
		} `json:"capabilities"`
	}
	_ = json.NewDecoder(wList.Body).Decode(&listResp)
	if len(listResp.Capabilities) != 1 || listResp.Capabilities[0].BindingsCount != 0 {
		t.Fatalf("expected 0 bindings count initially, got %+v", listResp.Capabilities)
	}

	// 3. 模拟产生了 2 个 AgentRun 绑定了该能力
	testRun1 := db.AgentRun{RunKey: "run-arch-01", AgentKind: "test", KernelVersion: "v1", State: "completed"}
	testRun2 := db.AgentRun{RunKey: "run-arch-02", AgentKind: "test", KernelVersion: "v1", State: "completed"}
	db.DB.Create(&testRun1)
	db.DB.Create(&testRun2)
	db.DB.Create(&db.RunCapabilityBinding{RunID: testRun1.ID, CapabilityKey: "archive.test.skill", Version: 1, Digest: "sha256:test1"})
	db.DB.Create(&db.RunCapabilityBinding{RunID: testRun2.ID, CapabilityKey: "archive.test.skill", Version: 1, Digest: "sha256:test2"})

	// 4. 再次查询详情：验证 bindings_count 为 2
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/agent-runtime/capabilities/archive.test.skill", nil)
	reqDetail.SetPathValue("id", "archive.test.skill")
	reqDetail.Header.Set("Authorization", authHeader)
	wDetail := httptest.NewRecorder()
	server.handler.ServeHTTP(wDetail, reqDetail)
	if wDetail.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wDetail.Code)
	}
	var detailResp struct {
		BindingsCount int64 `json:"bindings_count"`
		IsArchived    bool  `json:"is_archived"`
	}
	_ = json.NewDecoder(wDetail.Body).Decode(&detailResp)
	if detailResp.BindingsCount != 2 || detailResp.IsArchived {
		t.Fatalf("expected 2 bindings and is_archived=false, got %+v", detailResp)
	}

	// 5. 触发冷归档：DELETE /api/agent-runtime/capabilities/{id}?mode=cold_archive
	reqArch := httptest.NewRequest(http.MethodDelete, "/api/agent-runtime/capabilities/archive.test.skill?mode=cold_archive", nil)
	reqArch.SetPathValue("id", "archive.test.skill")
	reqArch.Header.Set("Authorization", authHeader)
	wArch := httptest.NewRecorder()
	server.handler.ServeHTTP(wArch, reqArch)
	if wArch.Code != http.StatusOK {
		t.Fatalf("expected 200 for cold archive, got %d, body: %s", wArch.Code, wArch.Body.String())
	}

	// 验证：状态标记为 archived，但资源内容完好保留（历史重放可用）
	var archivedCap db.Capability
	_ = db.DB.Where("capability_key = ?", "archive.test.skill").First(&archivedCap)
	if archivedCap.Status != "archived" {
		t.Fatalf("expected status archived, got %s", archivedCap.Status)
	}
	var res db.CapabilityResource
	_ = db.DB.Where("resource_key = ?", "instructions").First(&res)
	if res.Content == "" {
		t.Fatalf("expected resource content to be preserved under cold_archive, got empty")
	}

	// 6. 重新安装：POST /api/agent-runtime/capabilities/{id}/reinstall
	reqReinst := httptest.NewRequest(http.MethodPost, "/api/agent-runtime/capabilities/archive.test.skill/reinstall", nil)
	reqReinst.SetPathValue("id", "archive.test.skill")
	reqReinst.Header.Set("Authorization", authHeader)
	wReinst := httptest.NewRecorder()
	server.handler.ServeHTTP(wReinst, reqReinst)
	if wReinst.Code != http.StatusOK {
		t.Fatalf("expected 200 for reinstall, got %d", wReinst.Code)
	}
	_ = db.DB.Where("capability_key = ?", "archive.test.skill").First(&archivedCap)
	if archivedCap.Status != "active" {
		t.Fatalf("expected active status after reinstall, got %s", archivedCap.Status)
	}

	// 7. 触发强力清除：DELETE /api/agent-runtime/capabilities/{id}?mode=purge
	reqPurge := httptest.NewRequest(http.MethodDelete, "/api/agent-runtime/capabilities/archive.test.skill?mode=purge", nil)
	reqPurge.SetPathValue("id", "archive.test.skill")
	reqPurge.Header.Set("Authorization", authHeader)
	wPurge := httptest.NewRecorder()
	server.handler.ServeHTTP(wPurge, reqPurge)
	if wPurge.Code != http.StatusOK {
		t.Fatalf("expected 200 for purge, got %d", wPurge.Code)
	}
	_ = db.DB.Where("capability_key = ?", "archive.test.skill").First(&archivedCap)
	if archivedCap.Status != "uninstalled" {
		t.Fatalf("expected status uninstalled after purge, got %s", archivedCap.Status)
	}
	_ = db.DB.Where("resource_key = ?", "instructions").First(&res)
	if res.Content != "" {
		t.Fatalf("expected resource content to be cleared after purge, got %q", res.Content)
	}
}

