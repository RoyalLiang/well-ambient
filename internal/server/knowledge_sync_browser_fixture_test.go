package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

func TestKnowledgeSyncBrowserFixture(t *testing.T) {
	if os.Getenv("WELL_KNOWLEDGE_SYNC_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in isolated authenticated UI fixture")
	}
	srv, _ := setupAIGovernanceOpenCapabilityServer(t)
	configureJiraIssueBrowserFixture(t, srv)
	srv.config.Jira.SyncProjects = []string{"NS2", "WA"}
	srv.config.GitLab.Repos = []config.RepoMapping{{Name: "terminal", Path: "ops/terminal", ProjectID: "42"}, {Name: "fleet", Path: "ops/fleet", ProjectID: "43"}, {Name: "elsewhere", Path: "other/app", ProjectID: "99"}}
	srv.setEmailConfig(*srv.config)
	token := superAdminToken(t, "preview-admin@example.test", "预览管理员", []string{"*"})
	memberToken := performanceMemberToken(t, "sync-fixture-member@example.test", []string{"config:read"})
	for i := 0; i < 58; i++ {
		member := userdb.User{Username: fmt.Sprintf("member-%02d@example.test", i), Email: fmt.Sprintf("member-%02d@example.test", i), Name: fmt.Sprintf("成员 %02d", i)}
		if err := db.DB.Create(&member).Error; err != nil {
			t.Fatal(err)
		}
		if i < 4 {
			var group userdb.UserGroup
			db.DB.Where("name = ?", "member").First(&group)
			db.DB.Create(&userdb.UserGroupMembership{UserID: member.ID, UserGroupID: group.ID, Scope: "global"})
		}
	}
	for i := 1; i <= 4; i++ {
		db.DB.Create(&db.TaskTelemetry{TaskID: fmt.Sprintf("NS2-%d", i), ProjectKey: "NS2", Source: "jira", Title: fmt.Sprintf("车辆反馈与资源锁场景 %d", i), Description: "这是隔离验收的合成 Jira 内容，需要匹配任务和车辆反馈后才能确认完成。", Status: "progress", SourceUpdatedAt: time.Now()})
	}
	db.DB.Create(&db.ContextDocument{Title: "资源锁能力边界", Content: "全局调度锁不代表资源级并行能力。需要检查版本、租约和 fencing。", Scope: "global", Status: "active", IngestionStatus: "completed", Version: 1})
	if _, err := srv.runtimeKnowledge().Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	dist, _ := filepath.Abs("../../web/dist")
	files := http.FileServer(http.Dir(dist))
	tokenJSON, _ := json.Marshal(token)
	memberJSON, _ := json.Marshal(memberToken)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__fixture/login" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.URL.Query().Get("role") == "member" {
				fmt.Fprintf(w, `<script>localStorage.setItem('jwt_token',%s);localStorage.setItem('current_user_name','只读成员');localStorage.setItem('current_user_email','sync-fixture-member@example.test');localStorage.setItem('current_user_role','member');localStorage.setItem('current_user_permissions','["config:read"]');location.replace('/?tab=settings&section=jira_issue_sync');</script>`, memberJSON)
				return
			}
			fmt.Fprintf(w, `<script>localStorage.setItem('jwt_token',%s);localStorage.setItem('current_user_name','预览管理员');localStorage.setItem('current_user_email','preview-admin@example.test');localStorage.setItem('current_user_role','super_admin');localStorage.setItem('current_user_permissions','["*"]');location.replace('/?tab=ai_governance&section=skill_registry');</script>`, tokenJSON)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/open/") {
			srv.handler.ServeHTTP(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	host := &http.Server{Addr: "127.0.0.1:18092", Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(func() { host.Close() })
	t.Log("isolated preview http://127.0.0.1:18092/__fixture/login")
	if err := host.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
