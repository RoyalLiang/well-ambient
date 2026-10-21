package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"well-ambient/internal/config"
)

func TestGitLabInstancesBrowserFixture(t *testing.T) {
	if os.Getenv("WELL_FDE_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in isolated authenticated fixture")
	}
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	var mu sync.Mutex
	hooks := map[string]bool{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("PRIVATE-TOKEN") != "fixture-fde" {
			http.Error(w, "wrong instance credential", 401)
			return
		}
		switch {
		case r.URL.Path == "/api/v4/user":
			fmt.Fprint(w, `{"id":1}`)
		case strings.HasSuffix(r.URL.Path, "/projects"):
			projects := []map[string]any{}
			for i := 1; i <= 61; i++ {
				projects = append(projects, map[string]any{"id": i, "name": "task_executor", "path_with_namespace": fmt.Sprintf("project-code/prj%03d/task_executor", i), "namespace": map[string]any{"name": fmt.Sprintf("项目 %03d", i), "full_path": fmt.Sprintf("project-code/prj%03d", i)}})
			}
			json.NewEncoder(w).Encode(projects)
		case strings.HasSuffix(r.URL.Path, "/hooks"):
			mu.Lock()
			defer mu.Unlock()
			if r.Method == http.MethodPost {
				hooks[r.URL.Path] = true
				w.WriteHeader(201)
				fmt.Fprint(w, `{"id":1,"push_events":true,"merge_requests_events":true,"enable_ssl_verification":true}`)
				return
			}
			if hooks[r.URL.Path] {
				fmt.Fprintf(w, `[{"id":1,"url":"http://127.0.0.1:18093/api/webhook/gitlab/fde","push_events":true,"merge_requests_events":true,"enable_ssl_verification":true}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		default:
			http.Error(w, "unknown fixture endpoint", 404)
		}
	}))
	defer fake.Close()
	srv.config.GitLab = config.GitLabConfig{Enabled: true, BaseURL: "https://devgit.example.test", APIToken: "fixture-devgit", Secret: "fixture-devgit-hook", Connections: []config.GitLabConnection{{ID: "fde", Name: "FDE 项目代码", Enabled: true, BaseURL: fake.URL, APIToken: "fixture-fde", Secret: "fixture-fde-hook", RootGroup: "project-code"}}, Repos: []config.RepoMapping{{Name: "task_executor", Path: "core/task_executor", ProjectID: "1"}}}
	for i := 1; i <= 61; i++ {
		srv.config.GitLab.Repos = append(srv.config.GitLab.Repos, config.RepoMapping{Name: "task_executor", Path: fmt.Sprintf("project-code/prj%03d/task_executor", i), ProjectID: "fde:" + strconv.Itoa(i), InstanceID: "fde", RemoteProjectID: strconv.Itoa(i), NamespaceName: fmt.Sprintf("项目 %03d", i), NamespacePath: fmt.Sprintf("project-code/prj%03d", i)})
	}
	srv.setEmailConfig(*srv.config)
	token := superAdminToken(t, "preview-admin@example.test", "预览管理员", []string{"*"})
	tokenJSON, _ := json.Marshal(token)
	dist, _ := filepath.Abs("../../web/dist")
	files := http.FileServer(http.Dir(dist))
	host := &http.Server{Addr: "127.0.0.1:18093", ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__fixture/login" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<script>localStorage.setItem('jwt_token',%s);localStorage.setItem('current_user_name','预览管理员');localStorage.setItem('current_user_email','preview-admin@example.test');localStorage.setItem('current_user_role','super_admin');localStorage.setItem('current_user_permissions','["*"]');location.replace('/?tab=settings');</script>`, tokenJSON)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			srv.handler.ServeHTTP(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})}
	t.Cleanup(func() { host.Close() })
	t.Log("isolated FDE preview http://127.0.0.1:18093/__fixture/login")
	if err := host.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
