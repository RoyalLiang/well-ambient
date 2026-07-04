package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestAIOutputTraceReadModelReplaysArchiveContextPack(t *testing.T) {
	setupServerTestDB(t)
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9210, Host: "127.0.0.1"},
		AI: config.AIConfig{
			Model:                  "gpt-4.1-mini",
			DefaultWorkHoursPerDay: 7.5,
		},
	}, "")

	factHash := stableHash("权限策略事实\n策略层需要记录命中的 allow/deny 规则，并展示缺失权限。")
	if err := db.DB.Create(&db.ContextFact{
		Type:        "architecture",
		Scope:       "global",
		Source:      "manual",
		Status:      "active",
		Version:     3,
		Summary:     "权限策略事实",
		Content:     "策略层需要记录命中的 allow/deny 规则，并展示缺失权限。",
		TokenCount:  24,
		Freshness:   0.96,
		Confidence:  0.91,
		ContentHash: factHash,
	}).Error; err != nil {
		t.Fatalf("seed context fact: %v", err)
	}

	input := "为 RBAC 权限策略增加解释面板，需要说明命中的策略和缺失权限。"
	pack, err := srv.buildContextPack(db.DB, input, 600, "deconstruct", true)
	if err != nil {
		t.Fatalf("buildContextPack: %v", err)
	}

	result := DeconstructResponse{
		MappedRepos: []string{"platform-core"},
		Tasks: []TaskDetail{
			{
				ID:             "task-trace",
				Repo:           "platform-core",
				Title:          "实现权限策略解释 read model",
				Assignee:       "Eddie",
				Priority:       "High",
				Complexity:     "Medium",
				Difficulty:     "Medium",
				EstimatedHours: 12,
			},
		},
		Analysis: DeconstructAnalysis{
			CompletenessScore:     82,
			OverallEstimatedHours: 12,
			OverallDifficulty:     "Medium",
			MissingInfo:           []string{"确认普通成员是否可查看自己的权限解释"},
			Risks:                 []string{"权限误判会导致越权"},
			Dependencies:          []string{"现有策略授权日志"},
			AcceptanceCriteria:    []string{"权限拒绝时能展示命中策略和缺失权限"},
			MeetingQuestions:      []string{"是否需要展示 deny 策略优先级？"},
			Confidence:            0.73,
		},
	}
	normalizeDeconstructResponse(&result, false)
	archiveID, err := createDeconstructArchive(db.DB, input, "DEMAND-TRACE", "brain-demand-trace", pack.ID, result, time.Now())
	if err != nil {
		t.Fatalf("createDeconstructArchive: %v", err)
	}

	trace, err := srv.buildAIOutputTraceReadModel(db.DB, aiTraceQuery{ArchiveID: archiveID})
	if err != nil {
		t.Fatalf("buildAIOutputTraceReadModel: %v", err)
	}

	if trace.ArchiveID != archiveID || trace.ContextPackID != pack.ID {
		t.Fatalf("trace IDs = archive %d pack %d, want archive %d pack %d", trace.ArchiveID, trace.ContextPackID, archiveID, pack.ID)
	}
	if trace.InputSnapshot.Text != input || trace.InputSnapshot.TextHash == "" {
		t.Fatalf("input snapshot not preserved: %+v", trace.InputSnapshot)
	}
	if trace.ModelVersion != "gpt-4.1-mini" {
		t.Fatalf("ModelVersion = %q", trace.ModelVersion)
	}
	if trace.PromptTemplateVersion != contextPackTemplateV1 || trace.RuleVersion != aiOutputTraceRuleVersion {
		t.Fatalf("versions = prompt %q rule %q", trace.PromptTemplateVersion, trace.RuleVersion)
	}
	if trace.ContextPack == nil || !strings.Contains(trace.ContextPack.Summary, "权限策略事实") {
		t.Fatalf("context pack replay missing seeded fact: %+v", trace.ContextPack)
	}
	if len(trace.ReferencedFacts) == 0 || !strings.Contains(trace.ReferencedFacts[0].Content, "allow/deny") {
		t.Fatalf("referenced facts not replayed: %+v", trace.ReferencedFacts)
	}
	if len(trace.SourceVersions) == 0 || trace.SourceVersions[0].Version != 3 || trace.SourceVersions[0].ContentHash != factHash {
		t.Fatalf("source versions not preserved: %+v", trace.SourceVersions)
	}
	if trace.HumanFeedback.Status != "pending" {
		t.Fatalf("human feedback status = %q", trace.HumanFeedback.Status)
	}
}

func TestRequirementClarificationReadModelClassifiesQuestions(t *testing.T) {
	now := time.Now()
	archive := db.DeconstructArchive{
		ID:            42,
		DemandID:      "DEMAND-CLARIFY",
		TaskGroupID:   "brain-demand-clarify",
		ContextPackID: 7,
		Confidence:    0.52,
		CreatedAt:     now,
	}
	output := AITraceOutputSnapshot{
		Tasks: []TaskDetail{
			{ID: "task-clarify", Title: "实现需求澄清 read model"},
		},
		Analysis: DeconstructAnalysis{
			CompletenessScore: 58,
			MissingInfo: []string{
				"权限规则未明确",
				"数据口径未确定",
			},
			Risks: []string{
				"权限误判会导致越权",
			},
			Dependencies: []string{
				"Jira API 测试账号",
			},
			AcceptanceCriteria: []string{
				"澄清结果能展示必须回答、可后置和建议补充问题",
			},
			MeetingQuestions: []string{
				"是否需要灰度发布？",
				"建议补充运营文案",
			},
			Confidence: 0.52,
		},
	}

	clarification := buildRequirementClarificationFromOutput(&archive, output)
	if clarification.ReadinessScore != 58 || clarification.ReadinessLevel != "clarification_required" {
		t.Fatalf("readiness = %d/%s", clarification.ReadinessScore, clarification.ReadinessLevel)
	}
	if len(clarification.MissingQuestions.MustAnswer) != 3 {
		t.Fatalf("must-answer questions = %+v", clarification.MissingQuestions.MustAnswer)
	}
	if len(clarification.MissingQuestions.CanDefer) != 1 {
		t.Fatalf("can-defer questions = %+v", clarification.MissingQuestions.CanDefer)
	}
	if len(clarification.MissingQuestions.Suggested) != 1 {
		t.Fatalf("suggested questions = %+v", clarification.MissingQuestions.Suggested)
	}
	if len(clarification.AcceptanceCriteriaDraft) != 1 || !strings.Contains(clarification.AcceptanceCriteriaDraft[0], "澄清结果") {
		t.Fatalf("acceptance criteria draft = %+v", clarification.AcceptanceCriteriaDraft)
	}
	if len(clarification.RiskFlags) < 3 {
		t.Fatalf("risk flags should include explicit risk, low readiness, and must-answer blocker: %+v", clarification.RiskFlags)
	}
	if clarification.RiskFlags[0].Severity != "high" {
		t.Fatalf("first risk severity = %q", clarification.RiskFlags[0].Severity)
	}
}

func TestImportTasksAutoArchivesContextPackAndTrace(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "trace-import@westwell-lab.com", "Trace Import", []string{"demands:write"})
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9211, Host: "127.0.0.1"},
		AI: config.AIConfig{
			Model:               "gpt-4o-mini",
			ProjectArchitecture: "测试系统上下文",
		},
	}, "")
	useTempKanbanFile(t)

	now := time.Now()
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:        "DEMAND-AUTO-TRACE",
		Title:         "Demand without explicit input text",
		Repo:          "-",
		Assignee:      "Bob",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	payload := map[string]interface{}{
		"demand_id":   "DEMAND-AUTO-TRACE",
		"mappedRepos": []string{"platform-core"},
		"analysis": map[string]interface{}{
			"completeness_score":      76,
			"overall_estimated_hours": 10,
			"overall_difficulty":      "Medium",
			"missing_info":            []string{"验收口径待补充"},
			"risks":                   []string{},
			"dependencies":            []string{},
			"acceptance_criteria":     []string{"导入响应返回 archive_id 和 trace"},
			"schedule_notes":          []string{},
			"meeting_questions":       []string{},
			"confidence":              0.68,
		},
		"tasks": []map[string]interface{}{
			{
				"id":              "task-auto-trace",
				"repo":            "platform-core",
				"title":           "自动补齐 Context Pack 并返回 trace",
				"assignee":        "Bob",
				"priority":        "High",
				"complexity":      "Medium",
				"difficulty":      "Medium",
				"estimated_hours": 10,
			},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/import", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/tasks/import status = %d body %s", rr.Code, rr.Body.String())
	}

	var response struct {
		ArchiveID     uint                   `json:"archive_id"`
		ContextPackID uint                   `json:"context_pack_id"`
		Trace         AIOutputTraceReadModel `json:"trace"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if response.ArchiveID == 0 || response.ContextPackID == 0 {
		t.Fatalf("response should expose archive and context pack ids: %+v", response)
	}
	if response.Trace.ArchiveID != response.ArchiveID || response.Trace.ContextPackID != response.ContextPackID {
		t.Fatalf("trace IDs do not match response: %+v", response.Trace)
	}
	if !strings.Contains(response.Trace.InputSnapshot.Text, "自动补齐 Context Pack") {
		t.Fatalf("trace input snapshot should be derived from imported tasks: %q", response.Trace.InputSnapshot.Text)
	}

	var archive db.DeconstructArchive
	if err := db.DB.First(&archive, response.ArchiveID).Error; err != nil {
		t.Fatalf("archive not found: %v", err)
	}
	if archive.ContextPackID != response.ContextPackID || !strings.Contains(archive.InputText, "自动补齐 Context Pack") {
		t.Fatalf("archive context/input not persisted: %+v", archive)
	}

	var pack db.ContextPack
	if err := db.DB.First(&pack, response.ContextPackID).Error; err != nil {
		t.Fatalf("context pack not found: %v", err)
	}
	if pack.Purpose != "import_archive" {
		t.Fatalf("context pack purpose = %q, want import_archive", pack.Purpose)
	}
}
