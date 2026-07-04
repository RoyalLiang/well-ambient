package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func seedStrongestBrainUser(t *testing.T) string {
	t.Helper()
	if err := db.InitDB(":memory:"); err != nil {
		t.Fatalf("init db: %v", err)
	}
	u := userdb.User{
		Username:   "brain-user",
		Email:      "brain-user@westwell-lab.com",
		Name:       "Brain User",
		Department: "AI Platform",
	}
	if err := db.DB.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", "member").First(&group).Error; err != nil {
		t.Fatalf("query member group: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: u.ID, UserGroupID: group.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	token, err := GenerateJWT(u.Username, u.Name, "mock-token", "", []string{"member"}, []string{"dashboard:read", "demands:read", "decision:read"}, u.Department)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

func TestStrongestBrainDecisionQueueBuildsScheduleAndEvidenceItems(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	due := now.AddDate(0, 0, -2)
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-1",
		Title:         "逾期需求",
		IssueType:     "demand",
		Status:        "progress",
		Assignee:      "Brain User",
		Branch:        "feature/late",
		DueDate:       &due,
		TaskGroupID:   "brain-demand-1",
		TaskCreatedAt: now.AddDate(0, 0, -5),
		LastUpdate:    now.AddDate(0, 0, -4),
	}
	task := db.TaskTelemetry{
		TaskID:        "TASK-1",
		Title:         "已合并待回写",
		IssueType:     "task",
		Status:        "review",
		Assignee:      "Brain User",
		Repo:          "well-ambient",
		Branch:        "feature/late",
		TaskGroupID:   "brain-demand-1",
		TaskCreatedAt: now.AddDate(0, 0, -3),
		LastUpdate:    now.AddDate(0, 0, -2),
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    "TASK-1",
		Repo:      "well-ambient",
		Branch:    "feature/late",
		Action:    "mr_merge",
		MrURL:     "https://gitlab.example/mr/1",
		CreatedAt: now.Add(-2 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed git log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/decision-queue", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainDecisionQueueResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.Total < 2 {
		t.Fatalf("expected schedule and evidence decisions, got %+v", response.Summary)
	}
	if response.Summary.Critical == 0 {
		t.Fatalf("expected critical decision in %+v", response.Summary)
	}
}

func TestStrongestBrainDecisionQueueFlagsWeakSemanticEvidence(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	task := db.TaskTelemetry{
		TaskID:     "NS2-1692",
		Title:      "南沙二期配置中心点位",
		IssueType:  "task",
		Status:     "done",
		Assignee:   "梁志远",
		Repo:       "PRJ25151-南沙二期码头Q-Chassis运营20套 (NS2)",
		Branch:     "baiyun_dev",
		LastUpdate: now,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := db.DB.Create(&db.Notification{
		Type:      "semantic_linker",
		TaskID:    task.TaskID,
		Title:     "AI semantic link",
		Message:   "AI 自动将推送分支/Commit关联到未完成任务",
		Assignee:  "haoliang.jiang",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed semantic notification: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    task.TaskID,
		Repo:      "task_executor",
		Branch:    "baiyun_dev",
		CommitID:  "43ee9ba4ef611a45fec78fac46290a0f827be1cd",
		Message:   "feat: 适配新的配置中心点位",
		Author:    "haoliang.jiang",
		Action:    "git_push",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed git log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/decision-queue", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainDecisionQueueResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	found := false
	for _, item := range response.Items {
		if item.TaskID == task.TaskID && item.RiskType == "semantic_evidence_review" {
			found = true
			if item.RiskLevel != "critical" {
				t.Fatalf("risk level = %q, want critical", item.RiskLevel)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected semantic evidence review decision, got %+v", response.Items)
	}
}

func TestStrongestBrainDecisionQueueAddsEvidenceChainFieldsAndRules(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	dueSoon := now.AddDate(0, 0, 2)
	future := now.AddDate(0, 0, 10)
	staleUpdate := now.AddDate(0, 0, -5)

	rows := []db.TaskTelemetry{
		{
			TaskID:        "DEMAND-DUE",
			Title:         "临期证据不完整需求",
			IssueType:     "demand",
			Status:        "progress",
			Assignee:      "Brain User",
			Branch:        "feature/due",
			DueDate:       &dueSoon,
			TaskGroupID:   "brain-due",
			TaskCreatedAt: now.AddDate(0, 0, -4),
			LastUpdate:    now.AddDate(0, 0, -1),
		},
		{
			TaskID:        "TASK-DONE",
			Title:         "完成但无结果证据",
			IssueType:     "task",
			Status:        "done",
			Assignee:      "Brain User",
			Branch:        "feature/due",
			TaskGroupID:   "brain-due",
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    now.AddDate(0, 0, -1),
		},
		{
			TaskID:        "DEMAND-MR",
			Title:         "MR 已合并父需求",
			IssueType:     "demand",
			Status:        "progress",
			Assignee:      "Brain User",
			Branch:        "feature/mr",
			DueDate:       &future,
			TaskGroupID:   "brain-mr",
			TaskCreatedAt: now.AddDate(0, 0, -4),
			LastUpdate:    now,
		},
		{
			TaskID:        "TASK-MR",
			Title:         "MR 已合并但状态未完成",
			IssueType:     "task",
			Status:        "review",
			Assignee:      "Brain User",
			Branch:        "feature/mr",
			TaskGroupID:   "brain-mr",
			TaskCreatedAt: now.AddDate(0, 0, -3),
			LastUpdate:    now,
		},
		{
			TaskID:        "DEMAND-STALE",
			Title:         "已排期但无推进",
			IssueType:     "demand",
			Status:        "progress",
			Assignee:      "Brain User",
			Branch:        "feature/stale",
			DueDate:       &future,
			TaskGroupID:   "brain-stale",
			TaskCreatedAt: now.AddDate(0, 0, -8),
			LastUpdate:    staleUpdate,
		},
	}
	for _, row := range rows {
		if err := db.DB.Create(&row).Error; err != nil {
			t.Fatalf("seed task %s: %v", row.TaskID, err)
		}
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    "TASK-MR",
		Repo:      "well-ambient",
		Branch:    "feature/mr",
		Action:    "mr_merge",
		MrURL:     "https://gitlab.example/mr/42",
		CreatedAt: now.Add(-1 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed mr log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/decision-queue", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainDecisionQueueResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	doneWithoutEvidence := requireStrongestBrainDecision(t, response.Items, "TASK-DONE", "evidence_missing")
	if doneWithoutEvidence.DecisionOwner == "" || doneWithoutEvidence.Deadline == "" || doneWithoutEvidence.RecommendedAction == "" {
		t.Fatalf("done-without-evidence item missing decision fields: %+v", doneWithoutEvidence)
	}
	if doneWithoutEvidence.ChainStatus != "mismatch" || !strongestBrainStringsContain(doneWithoutEvidence.MissingLinks, "completion_evidence") {
		t.Fatalf("done-without-evidence chain fields mismatch: %+v", doneWithoutEvidence)
	}

	mrMismatch := requireStrongestBrainDecision(t, response.Items, "TASK-MR", "status_mismatch")
	if mrMismatch.ChainStatus != "mismatch" || mrMismatch.EvidenceCompleteness == 0 {
		t.Fatalf("MR mismatch chain fields missing: %+v", mrMismatch)
	}

	stale := requireStrongestBrainDecision(t, response.Items, "DEMAND-STALE", "stale_after_schedule")
	if stale.DecisionOwner != "Brain User" || stale.ChainStatus != "mismatch" {
		t.Fatalf("stale decision fields mismatch: %+v", stale)
	}

	dueSoonIncomplete := requireStrongestBrainDecision(t, response.Items, "DEMAND-DUE", "due_soon")
	if dueSoonIncomplete.EvidenceCompleteness >= 80 || !strongestBrainStringsContain(dueSoonIncomplete.MissingLinks, "merge_request") {
		t.Fatalf("due-soon incomplete chain not exposed: %+v", dueSoonIncomplete)
	}

	if response.Summary.EvidenceIncomplete == 0 || response.Summary.StatusMismatch == 0 || response.Summary.StaleAfterSchedule == 0 || response.Summary.DeadlineChainRisks == 0 {
		t.Fatalf("summary did not count evidence/status/stale/deadline risks: %+v", response.Summary)
	}
	if response.ExceptionSummary.ByType["evidence_missing"] == 0 || response.ExceptionSummary.ByType["status_mismatch"] == 0 {
		t.Fatalf("exception summary missing type counts: %+v", response.ExceptionSummary)
	}
	if len(response.WeeklyDecisions) == 0 || response.WeeklyDecisions[0].DecisionOwner == "" || len(response.WeeklyDecisions[0].EvidenceRefs) == 0 {
		t.Fatalf("weekly decisions missing read model fields: %+v", response.WeeklyDecisions)
	}
}

func TestStrongestBrainEvidenceChainReturnsRelatedTasksAndLogs(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	root := db.TaskTelemetry{
		TaskID:      "DEMAND-2",
		Title:       "需求证据链",
		IssueType:   "demand",
		Status:      "progress",
		TaskGroupID: "brain-demand-2",
		LastUpdate:  now,
	}
	child := db.TaskTelemetry{
		TaskID:      "TASK-2",
		Title:       "子任务",
		IssueType:   "task",
		Status:      "progress",
		TaskGroupID: "brain-demand-2",
		LastUpdate:  now,
	}
	if err := db.DB.Create(&root).Error; err != nil {
		t.Fatalf("seed root: %v", err)
	}
	if err := db.DB.Create(&child).Error; err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{TaskID: "TASK-2", Action: "git_push", CommitID: "abc123", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/evidence-chain?task_id=DEMAND-2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainEvidenceChainResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.RelatedTasks != 2 || response.Summary.Commits != 1 {
		t.Fatalf("unexpected evidence chain summary: %+v", response.Summary)
	}
	if response.ChainStatus == "" || response.EvidenceCompleteness == 0 || !strongestBrainStringsContain(response.MissingLinks, "merge_request") {
		t.Fatalf("evidence chain fields missing: %+v", response)
	}
}

func TestAIIntentSummaryDeterministicFallback(t *testing.T) {
	token := seedStrongestBrainUser(t)
	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"请总结本周逾期风险，并给出下次周会要问的问题"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/intent-summary", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response AIIntentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Intent != "summary" {
		t.Fatalf("intent = %q, want summary", response.Intent)
	}
	if response.Summary == "" || len(response.NextQuestions) == 0 || response.Source != "deterministic" {
		t.Fatalf("unexpected intent response: %+v", response)
	}
}

func TestStrongestBrainDeliveryCockpitAggregatesPhaseSignals(t *testing.T) {
	token := seedStrongestBrainUser(t)
	now := time.Now()
	due := now.AddDate(0, 0, -1)
	demand := db.TaskTelemetry{
		TaskID:        "DEMAND-DELIVERY",
		Title:         "交付驾驶舱需求",
		IssueType:     "demand",
		Status:        "progress",
		Assignee:      "Brain User",
		Branch:        "feature/delivery",
		DueDate:       &due,
		TaskGroupID:   "brain-delivery",
		TaskCreatedAt: now.AddDate(0, 0, -5),
		LastUpdate:    now.AddDate(0, 0, -4),
	}
	child := db.TaskTelemetry{
		TaskID:        "TASK-DELIVERY",
		Title:         "合并后待回写",
		IssueType:     "task",
		Status:        "review",
		Assignee:      "Brain User",
		Repo:          "well-ambient",
		Branch:        "feature/delivery",
		TaskGroupID:   "brain-delivery",
		TaskCreatedAt: now.AddDate(0, 0, -4),
		LastUpdate:    now.AddDate(0, 0, -3),
	}
	if err := db.DB.Create(&demand).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}
	if err := db.DB.Create(&child).Error; err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if err := db.DB.Create(&db.GitCommitLog{
		TaskID:    child.TaskID,
		Repo:      child.Repo,
		Branch:    child.Branch,
		Action:    "mr_merge",
		MrURL:     "https://gitlab.example/mr/7",
		CreatedAt: now.Add(-3 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed merge log: %v", err)
	}
	pack := db.ContextPack{
		Purpose:               "deconstruct",
		Model:                 "test-model",
		PromptTemplateVersion: "context-pack-v1",
		Summary:               "测试上下文包",
		TokenBudget:           1800,
		TokenCount:            32,
		ContextHash:           "ctx-hash",
		CreatedAt:             now,
	}
	if err := db.DB.Create(&pack).Error; err != nil {
		t.Fatalf("seed context pack: %v", err)
	}
	if err := db.DB.Create(&db.DeconstructArchive{
		DemandID:          demand.TaskID,
		TaskGroupID:       demand.TaskGroupID,
		ContextPackID:     pack.ID,
		InputText:         "实现交付驾驶舱，需要权限解释和 AI 回放",
		AnalysisJSON:      `{"completeness_score":66,"missing_info":["验收口径"],"risks":["权限边界不清"],"acceptance_criteria":["能看到 AI 回放"]}`,
		CompletenessScore: 66,
		Confidence:        0.72,
		CreatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("seed deconstruct archive: %v", err)
	}
	if err := db.DB.Create(&db.DecisionEvent{
		TaskID:    demand.TaskID,
		Actor:     "Brain User",
		Action:    "override_reassign",
		OldValue:  "Brain User",
		NewValue:  "朱家聪",
		Reason:    "临近交付需要协助",
		CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed decision event: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")
	req := httptest.NewRequest(http.MethodGet, "/api/strongest-brain/delivery-cockpit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StrongestBrainDeliveryCockpitResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Evidence.TotalRequirements != 1 || response.Evidence.MergedButStatusOpen == 0 {
		t.Fatalf("unexpected evidence summary: %+v", response.Evidence)
	}
	if response.Exceptions.P0 == 0 || response.WeeklyDecisions.Total == 0 {
		t.Fatalf("expected critical exception and weekly decision: exceptions=%+v weekly=%+v", response.Exceptions, response.WeeklyDecisions)
	}
	if response.AITrace.TraceableOutputs != 1 || len(response.AITrace.Latest) != 1 || response.AITrace.Latest[0].ContextPackID != pack.ID {
		t.Fatalf("unexpected AI trace summary: %+v", response.AITrace)
	}
	if response.Override.Recent == 0 || !response.Authorization.ExplainPanelAvailable {
		t.Fatalf("expected override and authorization summaries: override=%+v authorization=%+v", response.Override, response.Authorization)
	}
}

func TestStrongestBrainInterventionUpdatesTaskAndLogsEvent(t *testing.T) {
	_ = seedStrongestBrainUser(t) // 初始化 db 和基础用户

	var u userdb.User
	if err := db.DB.Where("username = ?", "brain-user").First(&u).Error; err != nil {
		t.Fatalf("query user: %v", err)
	}
	var superAdminGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "super_admin").First(&superAdminGroup).Error; err != nil {
		t.Fatalf("query super_admin group: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: u.ID, UserGroupID: superAdminGroup.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("seed super_admin membership: %v", err)
	}

	token, err := GenerateJWT(u.Username, u.Name, "mock-token", "", []string{"member", "super_admin"}, []string{"dashboard:read", "demands:read", "decision:read", "demands:write"}, u.Department)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	now := time.Now()
	task := db.TaskTelemetry{
		TaskID:     "DEMAND-3",
		Title:      "待转派需求",
		IssueType:  "demand",
		Status:     "progress",
		Assignee:   "Brain User",
		LastUpdate: now,
	}
	if err := db.DB.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	srv := NewServer(&config.Config{Server: config.ServerConfig{Host: "127.0.0.1", Port: 8080}}, "")

	// 测试 reassign 动作
	body := bytes.NewBufferString(`{
		"task_id": "DEMAND-3",
		"action": "reassign",
		"value": "朱家聪",
		"reason": "需要朱家聪协助攻坚"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/strongest-brain/intervention", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	// 验证数据库状态已修改
	var updatedTask db.TaskTelemetry
	if err := db.DB.Where("task_id = ?", "DEMAND-3").First(&updatedTask).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if updatedTask.Assignee != "朱家聪" {
		t.Fatalf("task assignee = %q, want %q", updatedTask.Assignee, "朱家聪")
	}

	// 验证 DecisionEvent 已经写入
	var event db.DecisionEvent
	if err := db.DB.Where("task_id = ?", "DEMAND-3").First(&event).Error; err != nil {
		t.Fatalf("query event: %v", err)
	}
	if event.Action != "override_reassign" || event.Actor != "Brain User" || event.NewValue != "朱家聪" || event.Reason != "需要朱家聪协助攻坚" {
		t.Fatalf("unexpected event log: %+v", event)
	}
}

func requireStrongestBrainDecision(t *testing.T, items []StrongestBrainDecisionItem, taskID string, riskType string) StrongestBrainDecisionItem {
	t.Helper()
	for _, item := range items {
		if item.TaskID == taskID && item.RiskType == riskType {
			return item
		}
	}
	t.Fatalf("decision %s/%s not found in %+v", taskID, riskType, items)
	return StrongestBrainDecisionItem{}
}

func strongestBrainStringsContain(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
