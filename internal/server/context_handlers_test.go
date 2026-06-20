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
	userdb "well-ambient/internal/db/user"
)

func TestContextFactCreateAndPackPreview(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "context-admin@westwell-lab.com", "Context Admin", []string{"ai_context:read", "ai_context:write", "ai_context:preview"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9200, Host: "127.0.0.1"}}, "")

	payload := map[string]interface{}{
		"type":       "architecture",
		"scope":      "global",
		"source":     "manual",
		"status":     "active",
		"summary":    "RBAC 架构边界",
		"content":    "权限系统由用户组矩阵提供兼容授权，策略层提供显式 allow/deny 覆盖。",
		"freshness":  0.9,
		"confidence": 0.92,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/context/facts", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/context/facts status = %d body %s", rr.Code, rr.Body.String())
	}

	var fact db.ContextFact
	if err := db.DB.Where("summary = ?", "RBAC 架构边界").First(&fact).Error; err != nil {
		t.Fatalf("context fact not persisted: %v", err)
	}
	if fact.TokenCount <= 0 || fact.ContentHash == "" {
		t.Fatalf("fact should have token count and hash: %+v", fact)
	}

	previewPayload := map[string]interface{}{
		"demand_text":  "重构 RBAC 权限系统，需要保留用户组矩阵并加入策略解释",
		"token_budget": 400,
	}
	body, _ = json.Marshal(previewPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/context/pack/preview", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/context/pack/preview status = %d body %s", rr.Code, rr.Body.String())
	}

	var response struct {
		Pack contextPackDTO `json:"pack"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if len(response.Pack.Items) == 0 || !strings.Contains(response.Pack.Summary, "RBAC") {
		t.Fatalf("preview did not select seeded fact: %+v", response.Pack)
	}
	if response.Pack.CacheKey == "" || response.Pack.ContextHash == "" {
		t.Fatalf("preview should return cache/hash keys: %+v", response.Pack)
	}
}

func TestImportTasksArchivesContextPackID(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "archive-context@westwell-lab.com", "Archive Context", []string{"demands:write"})
	srv := NewServer(&config.Config{
		Server: config.ServerConfig{Port: 9201, Host: "127.0.0.1"},
		AI:     config.AIConfig{ProjectArchitecture: "测试上下文兜底"},
	}, "")
	useTempKanbanFile(t)

	now := time.Now()
	if err := db.DB.Create(&db.TaskTelemetry{
		TaskID:        "DEMAND-CTX",
		Title:         "Demand with context pack",
		Repo:          "-",
		Assignee:      "Bob",
		Status:        "backlog",
		IssueType:     "demand",
		TaskCreatedAt: now,
		LastUpdate:    now,
	}).Error; err != nil {
		t.Fatalf("seed demand: %v", err)
	}

	pack, err := srv.buildContextPack(db.DB, "需要结构化上下文归档", 500, "test", true)
	if err != nil {
		t.Fatalf("buildContextPack: %v", err)
	}
	if pack.ID == 0 {
		t.Fatalf("pack should be persisted")
	}

	payload := map[string]interface{}{
		"demand_id":       "DEMAND-CTX",
		"context_pack_id": pack.ID,
		"input_text":      "需要结构化上下文归档",
		"mappedRepos":     []string{"platform-core"},
		"analysis": map[string]interface{}{
			"completeness_score":      80,
			"overall_estimated_hours": 16,
			"overall_difficulty":      "Medium",
			"missing_info":            []string{},
			"risks":                   []string{},
			"dependencies":            []string{},
			"acceptance_criteria":     []string{},
			"schedule_notes":          []string{},
			"meeting_questions":       []string{},
			"confidence":              0.7,
		},
		"tasks": []map[string]interface{}{
			{
				"id":              "task-ctx",
				"repo":            "platform-core",
				"title":           "Persist context pack linkage",
				"assignee":        "Bob",
				"priority":        "High",
				"complexity":      "Medium",
				"difficulty":      "Medium",
				"estimated_hours": 16,
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

	var archive db.DeconstructArchive
	if err := db.DB.Where("demand_id = ?", "DEMAND-CTX").First(&archive).Error; err != nil {
		t.Fatalf("archive not found: %v", err)
	}
	if archive.ContextPackID != pack.ID {
		t.Fatalf("ContextPackID = %d, want %d", archive.ContextPackID, pack.ID)
	}
}

func TestAuthorizationPolicyAPIs(t *testing.T) {
	setupServerTestDB(t)
	token := superAdminToken(t, "policy-api@westwell-lab.com", "Policy API", []string{"policies:read", "policies:write", "authorization_audit:read"})
	srv := NewServer(&config.Config{Server: config.ServerConfig{Port: 9202, Host: "127.0.0.1"}}, "")

	member := userdb.User{
		Username: "policy-target@westwell-lab.com",
		Email:    "policy-target@westwell-lab.com",
		Name:     "Policy Target",
	}
	if err := db.DB.Create(&member).Error; err != nil {
		t.Fatalf("seed policy target: %v", err)
	}
	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", "member").First(&group).Error; err != nil {
		t.Fatalf("query member group: %v", err)
	}
	if err := db.DB.Create(&userdb.UserGroupMembership{UserID: member.ID, UserGroupID: group.ID, Scope: "global"}).Error; err != nil {
		t.Fatalf("bind member group: %v", err)
	}

	payload := map[string]interface{}{
		"effect":        "allow",
		"subject_type":  "user",
		"subject_id":    "policy-target@westwell-lab.com",
		"action":        "config:read",
		"resource_type": "config",
		"scope":         "global",
		"priority":      10,
		"enabled":       true,
		"reason":        "temporary config review",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/authz/policies", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/authz/policies status = %d body %s", rr.Code, rr.Body.String())
	}

	explain := map[string]interface{}{
		"username":      "policy-target@westwell-lab.com",
		"action":        "config:read",
		"resource_type": "config",
		"scope":         "global",
	}
	body, _ = json.Marshal(explain)
	req = httptest.NewRequest(http.MethodPost, "/api/authz/explain", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/authz/explain status = %d body %s", rr.Code, rr.Body.String())
	}

	var decision struct {
		Allowed bool   `json:"allowed"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if !decision.Allowed || decision.Reason != "temporary config review" {
		t.Fatalf("decision = %+v, want allow with policy reason", decision)
	}
}
