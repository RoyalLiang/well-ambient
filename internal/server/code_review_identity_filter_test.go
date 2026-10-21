package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestCodeReviewAcceptsRegisteredEmailIdentityWithGrantedPermissions(t *testing.T) {
	srv, _, _ := reviewFixtureServer(t)
	seedPolicyTestUser(t, "jiacong.zhu", "admin", "global", "")
	if err := db.DB.Model(&userdb.User{}).Where("username = ?", "jiacong.zhu").Update("email", "jiacong.zhu@westwell-lab.com").Error; err != nil {
		t.Fatal(err)
	}
	token, err := GenerateJWT("jiacong.zhu@westwell-lab.com", "朱家聪", "fixture-only", "", []string{"admin"}, []string{"dashboard:read", "ai_context:preview"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/me", "/api/code-reviews/repos", "/api/code-reviews"} {
		t.Run(path, func(t *testing.T) {
			response := reviewRequest(srv, token, http.MethodGet, path, "")
			if response.Code != http.StatusOK {
				t.Fatalf("registered email identity rejected at %s: %d %s", path, response.Code, response.Body.String())
			}
		})
	}
	// A signed but unrelated email must not inherit the prefix user's access.
	foreign, err := GenerateJWT("jiacong.zhu@other.example", "Unknown", "", "", []string{"admin"}, []string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	if response := reviewRequest(srv, foreign, http.MethodGet, "/api/code-reviews", ""); response.Code != http.StatusForbidden {
		t.Fatalf("unregistered foreign email inherited privileges: %d", response.Code)
	}
}

func TestDegradedLoginEmailRequiresMatchingCachedPassword(t *testing.T) {
	srv, _, _ := reviewFixtureServer(t)
	seedPolicyTestUser(t, "jiacong.zhu", "admin", "global", "")
	hash, err := deriveLocalPasswordHash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Model(&userdb.User{}).Where("username = ?", "jiacong.zhu").Updates(map[string]any{"email": "jiacong.zhu@westwell-lab.com", "local_password_hash": hash}).Error; err != nil {
		t.Fatal(err)
	}
	for _, password := range []string{"wrong-password", "fixture-password"} {
		r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		w := httptest.NewRecorder()
		srv.handleDegradedLogin(w, r, "jiacong.zhu@westwell-lab.com", password, errors.New("fixture upstream unavailable"))
		if password == "wrong-password" {
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("wrong password accepted: %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusOK {
			t.Fatalf("email fallback rejected: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		claims, err := ParseJWT(body.Token)
		if err != nil || claims.UserID != "jiacong.zhu" {
			t.Fatalf("fallback identity not canonical: claims=%+v err=%v", claims, err)
		}
	}
}

func TestLoginEmailRetainsRegisteredAccountAndPermissions(t *testing.T) {
	srv, _, _ := reviewFixtureServer(t)
	seedPolicyTestUser(t, "jiacong.zhu", "admin", "global", "")
	if err := db.DB.Model(&userdb.User{}).Where("username = ?", "jiacong.zhu").Update("email", "jiacong.zhu@westwell-lab.com").Error; err != nil {
		t.Fatal(err)
	}
	oldLogin, oldInfo := wellOSLoginDoer, wellOSUserInfoDoer
	t.Cleanup(func() { wellOSLoginDoer, wellOSUserInfoDoer = oldLogin, oldInfo })
	wellOSLoginDoer = func(*http.Client, string, string) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"token":"fixture-wellos","name":"朱家聪"}`))}, nil
	}
	wellOSUserInfoDoer = func(*http.Client, string) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"email":"jiacong.zhu@westwell-lab.com"}}`))}, nil
	}
	response := reviewRequest(srv, "", http.MethodPost, "/api/login", `{"username":"jiacong.zhu@westwell-lab.com","password":"fixture-only"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("email login failed: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Token string `json:"token"`
		User  struct {
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	claims, err := ParseJWT(body.Token)
	if err != nil || claims.UserID != "jiacong.zhu" {
		t.Fatalf("login did not bind persisted identity: user=%q err=%v", claims.UserID, err)
	}
	if !strings.Contains(strings.Join(body.User.Permissions, ","), "dashboard:read") {
		t.Fatal("registered account's granted permissions disappeared at login")
	}
	if got := reviewRequest(srv, body.Token, http.MethodGet, "/api/code-reviews", ""); got.Code != http.StatusOK {
		t.Fatalf("new login cannot open review page: %d", got.Code)
	}
}
