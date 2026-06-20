package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/server/authz"
)

func TestAuthorizationSeedsFineGrainedPermissionsAndMigratesModels(t *testing.T) {
	setupServerTestDB(t)

	for _, code := range []string{
		"ai_context:read",
		"ai_context:write",
		"ai_context:preview",
		"policies:read",
		"policies:write",
		"authorization_audit:read",
	} {
		var count int64
		if err := db.DB.Model(&userdb.Permission{}).Where("code = ?", code).Count(&count).Error; err != nil {
			t.Fatalf("Failed to query permission %s: %v", code, err)
		}
		if count != 1 {
			t.Fatalf("permission %s count = %d, want 1", code, count)
		}
	}

	policy := userdb.AuthorizationPolicy{
		Effect:       "deny",
		SubjectType:  "any",
		Action:       "config:write",
		ResourceType: "config",
		Scope:        "global",
		Enabled:      true,
		Reason:       "test migration insert",
	}
	if err := db.DB.Create(&policy).Error; err != nil {
		t.Fatalf("authorization_policies model was not migrated: %v", err)
	}

	audit := userdb.AuthorizationAuditLog{
		SubjectUsername: "seed-check@westwell-lab.com",
		Action:          "config:write",
		ResourceType:    "config",
		Scope:           "global",
		Allowed:         false,
		Reason:          "test migration insert",
		RiskLevel:       "high",
	}
	if err := db.DB.Create(&audit).Error; err != nil {
		t.Fatalf("authorization_audit_logs model was not migrated: %v", err)
	}
}

func TestHandleListPermissionsReturnsSeededCatalog(t *testing.T) {
	setupServerTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/permissions", nil)
	rr := httptest.NewRecorder()

	(&Server{}).handleListPermissions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var permissions []PermissionDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &permissions); err != nil {
		t.Fatalf("decode permissions response: %v", err)
	}

	if len(permissions) == 0 {
		t.Fatalf("permissions response should not be empty")
	}

	var found bool
	for _, perm := range permissions {
		if perm.Code == "policies:write" {
			found = true
			if perm.Name == "" || perm.Description == "" {
				t.Fatalf("policies:write should include name and description: %+v", perm)
			}
		}
	}
	if !found {
		t.Fatalf("permissions response missing policies:write: %+v", permissions)
	}
}

func TestWithPermissionExplicitDenyOverridesLegacyGroupPermission(t *testing.T) {
	setupServerTestDB(t)
	token := seedPolicyTestUser(t, "denied-admin@westwell-lab.com", "admin", "global", "")

	policy := userdb.AuthorizationPolicy{
		Effect:       "deny",
		SubjectType:  "user",
		SubjectID:    "denied-admin@westwell-lab.com",
		Action:       "config:read",
		ResourceType: "config",
		Scope:        "global",
		Priority:     100,
		Enabled:      true,
		Reason:       "configuration freeze",
	}
	if err := db.DB.Create(&policy).Error; err != nil {
		t.Fatalf("Failed to seed deny policy: %v", err)
	}

	handler := (&Server{}).withPermission("config:read", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected-config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body %s", rr.Code, http.StatusForbidden, rr.Body.String())
	}

	var audit userdb.AuthorizationAuditLog
	if err := db.DB.Where("subject_username = ? AND action = ?", "denied-admin@westwell-lab.com", "config:read").
		First(&audit).Error; err != nil {
		t.Fatalf("Expected denied decision audit log: %v", err)
	}
	if audit.Allowed {
		t.Fatalf("audit.Allowed = true, want denied audit")
	}
	if audit.MatchedPolicyID == nil || *audit.MatchedPolicyID != policy.ID {
		t.Fatalf("MatchedPolicyID = %v, want %d", audit.MatchedPolicyID, policy.ID)
	}
	if audit.Reason != "configuration freeze" {
		t.Fatalf("audit.Reason = %q, want deny reason", audit.Reason)
	}
}

func TestWithPermissionPreservesRepoScopedCompatibility(t *testing.T) {
	setupServerTestDB(t)
	token := seedPolicyTestUser(t, "repo-admin@westwell-lab.com", "admin", "repo", "platform-core")

	handler := (&Server{}).withPermission("config:read", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected-config?repo=platform-core", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("matching repo status = %d, want %d; body %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/protected-config?repo=other-repo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("other repo status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	req = httptest.NewRequest(http.MethodGet, "/protected-config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("global request status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestAuthorizeAllowsPolicyGrantWithoutGroupPermission(t *testing.T) {
	setupServerTestDB(t)
	seedPolicyTestUser(t, "policy-member@westwell-lab.com", "member", "global", "")

	decision := authz.NewEvaluator(db.DB).Authorize(
		nil,
		authz.Subject{Username: "policy-member@westwell-lab.com"},
		"config:read",
		authz.Resource{Type: "config", Scope: "global"},
	)
	if decision.Allowed {
		t.Fatalf("member config:read allowed before policy grant")
	}

	if err := db.DB.Create(&userdb.AuthorizationPolicy{
		Effect:       "allow",
		SubjectType:  "group",
		SubjectID:    "member",
		Action:       "config:read",
		ResourceType: "config",
		Scope:        "global",
		Priority:     10,
		Enabled:      true,
		Reason:       "temporary read-only config review",
	}).Error; err != nil {
		t.Fatalf("Failed to seed allow policy: %v", err)
	}

	decision = authz.NewEvaluator(db.DB).Authorize(
		nil,
		authz.Subject{Username: "policy-member@westwell-lab.com"},
		"config:read",
		authz.Resource{Type: "config", Scope: "global"},
	)
	if !decision.Allowed {
		t.Fatalf("policy grant denied: %+v", decision)
	}
	if decision.MatchedPolicy == nil || decision.Reason != "temporary read-only config review" {
		t.Fatalf("decision did not explain matched allow policy: %+v", decision)
	}
}

func seedPolicyTestUser(t *testing.T, username, groupName, scope, scopeID string) string {
	t.Helper()

	u := userdb.User{
		Username: username,
		Email:    username,
		Name:     username,
	}
	if err := db.DB.Create(&u).Error; err != nil {
		t.Fatalf("Failed to seed policy test user: %v", err)
	}

	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", groupName).First(&group).Error; err != nil {
		t.Fatalf("Failed to query group %s: %v", groupName, err)
	}

	if err := db.DB.Create(&userdb.UserGroupMembership{
		UserID:      u.ID,
		UserGroupID: group.ID,
		Scope:       scope,
		ScopeID:     scopeID,
	}).Error; err != nil {
		t.Fatalf("Failed to bind policy test membership: %v", err)
	}

	token, err := GenerateJWT(username, username, "mock_wellos_token", "", []string{groupName}, nil)
	if err != nil {
		t.Fatalf("Failed to generate policy test token: %v", err)
	}
	return token
}
