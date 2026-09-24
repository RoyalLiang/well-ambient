package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/mailreport"
)

// Explicitly opt-in fixture: an in-memory database, local SMTP sink and deterministic
// AI response. It never loads production configuration or starts background workers.
func TestEmailBrowserFixture(t *testing.T) {
	if os.Getenv("WELL_EMAIL_BROWSER_FIXTURE") != "1" {
		t.Skip("manual authenticated browser fixture")
	}
	var confluenceStub *emailConfluenceBrowserStub
	if os.Getenv("WELL_EMAIL_CONFLUENCE_FIXTURE") == "1" {
		confluenceStub = newEmailConfluenceBrowserStub(t)
	}
	setupServerTestDB(t)
	sqlDB, _ := db.DB.DB()
	defer sqlDB.Close()
	smtpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer smtpListener.Close()
	_, portText, _ := net.SplitHostPort(smtpListener.Addr().String())
	port, _ := strconv.Atoi(portText)
	var accepted atomic.Int32
	go func() {
		for {
			conn, err := smtpListener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				reader := textproto.NewReader(bufio.NewReader(conn))
				reply := func(s string) { _, _ = fmt.Fprint(conn, s+"\r\n") }
				reply("220 local test SMTP")
				for {
					line, err := reader.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
						reply("250 local")
					case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
						reply("250 ok")
					case line == "DATA":
						reply("354 data")
						if _, err := reader.ReadDotBytes(); err != nil {
							return
						}
						accepted.Add(1)
						reply("250 accepted")
					case line == "QUIT":
						reply("221 bye")
						return
					default:
						reply("500 unsupported")
					}
				}
			}()
		}
	}()
	token := superAdminToken(t, "mail-fixture@example.test", "Mail Fixture", []string{"config:read", "config:write", "users:read"})
	readerToken := seedPolicyTestUser(t, "mail-reader@example.test", "member", "global", "")
	var memberGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "member").First(&memberGroup).Error; err != nil {
		t.Fatal(err)
	}
	var readPermission userdb.Permission
	if err := db.DB.Where("code = ?", "config:read").First(&readPermission).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.FirstOrCreate(&userdb.GroupPermission{}, userdb.GroupPermission{UserGroupID: memberGroup.ID, PermissionID: readPermission.ID}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SMTP: config.SMTPConfig{Host: "127.0.0.1", Port: port, TLSMode: "none", From: "fixture@example.test"}, Jira: config.JiraConfig{BaseURL: "https://jira.example.test/jira", SyncUsers: []string{"fixture"}}, DailyJiraEmail: config.DailyJiraEmailConfig{ProjectGroups: []config.DailyJiraProjectGroup{{Name: "邮件验证组", Projects: []string{"MAIL"}, Owners: []string{"fixture"}}}, Recipients: []string{"fixture@example.test"}, Timezone: "Asia/Shanghai", SendTime: "09:00", Template: config.EmailTemplate{Subject: "Jira 每日早报 · {{date}}"}}}
	if err := db.DB.Create(&userdb.User{Username: "fixture", Name: "Fixture", Email: "fixture@example.test"}).Error; err != nil {
		t.Fatal(err)
	}
	today, yesterday, _, _ := emailDateBounds("", "Asia/Shanghai", time.Now())
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "MAIL-1", Title: "FMS 验证 Jira 邮件早报", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Status: "progress", Assignee: "External owner", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.JiraReportChange{TaskID: "MAIL-1", HistoryID: "fixture-transfer", Field: "assignee", FromID: "fixture", FromValue: "Fixture", ToID: "external", ToValue: "External owner", OccurredAt: yesterday.Add(30 * time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.TaskTelemetry{TaskID: "MAIL-2", Title: "FMS 标题但属于硬件", Source: "jira", JiraBugCategory: "硬件", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Assignee: "Fixture", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.GitCommitLog{Repo: "fixture", Branch: "main", Message: "修正日报统计与跳转链接", CommitID: "fixture-sha", Author: "Fixture", Action: "git_push", CreatedAt: today.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	cfg.GitLab = config.GitLabConfig{BaseURL: "https://gitlab.example.test", Repos: []config.RepoMapping{{Name: "fixture", Path: "team/fixture"}}}
	cfg.DailyJiraEmail.IncludeCommits = true
	if err := BootstrapVersionedConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(&cfg, "")
	srv.emailLLM = func(ctx context.Context, system, user string) (string, error) {
		if strings.Contains(user, "失败") {
			return "", fmt.Errorf("fixture failure")
		}
		if strings.Contains(user, "等待") {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		return `{"style":"custom","subject":"团队早报 {{date}}","introduction":"请关注下列未解决事项与负责人。","closing":"请负责人确认跟进计划。","html":"<div><h1>团队早报</h1><p>{{introduction}}</p><p>{{date}} · {{timezone}}</p>{{yesterday}}{{unresolved}}<p>{{closing}}</p></div>"}`, nil
	}
	srv.emailSender = func(ctx context.Context, c config.SMTPConfig, to []string, subject, text, html string) error {
		if c.Host != "127.0.0.1" || c.Port != port {
			return fmt.Errorf("fixture permits only local SMTP sink")
		}
		if confluenceStub != nil {
			confluenceStub.mu.Lock()
			confluenceStub.sends++
			confluenceStub.sentHTML = html
			confluenceStub.mu.Unlock()
		}
		return mailreport.Send(ctx, c, to, subject, text, html)
	}
	root, err := filepath.Abs("../../web/dist")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&db.DailyJiraEmailRun{Date: "2026-01-02", Status: "sent", ConfluenceStatus: "failed", ConfluenceError: "fixture sync unavailable", RecipientsJSON: `["recipient@example.test"]`}).Error; err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv.handler)
	mux.Handle("/", http.FileServer(http.Dir(root)))
	stop := make(chan struct{})
	var once sync.Once
	mux.HandleFunc("GET /__fixture/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><script>localStorage.setItem('jwt_token',%q);localStorage.setItem('current_user_name','Email Fixture');localStorage.setItem('current_user_email','fixture@example.test');localStorage.setItem('current_user_role','super_admin');localStorage.setItem('current_user_permissions','["dashboard:read","config:read","config:write"]');location.replace('/');</script>`, token)
	})
	mux.HandleFunc("GET /__fixture/chart-preview", func(w http.ResponseWriter, r *http.Request) {
		report, err := emailTemplateDemo(config.DefaultEmailTemplate())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, report.HTML)
	})

	mux.HandleFunc("GET /__fixture/session", func(w http.ResponseWriter, r *http.Request) {
		session := map[string]any{"token": token, "reader_token": readerToken, "smtp_port": port, "accepted": accepted.Load()}
		if confluenceStub != nil {
			session["confluence_url"] = confluenceStub.server.URL + "/display/~fixture/well-infra"
			session["confluence_token"] = "fixture-confluence-token"
		}
		emailJSON(w, 200, session)
	})
	if confluenceStub != nil {
		mux.HandleFunc("/__fixture/confluence", confluenceStub.inspect)
	}
	mux.HandleFunc("POST /__fixture/sync-project", func(w http.ResponseWriter, r *http.Request) {
		task := db.TaskTelemetry{TaskID: "NEWJIRA-1", ProjectKey: "NEWJIRA", Source: "jira", JiraBugCategory: "FMS", JiraBugCategoryFieldID: "customfield_fixture", JiraHistoryComplete: true, Title: "New synchronized project", Repo: "New delivery (NEWJIRA)", Assignee: "Fixture", Status: "progress", TaskCreatedAt: yesterday, SourceUpdatedAt: yesterday}
		if err := db.DB.Save(&task).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		BroadcastTelemetryUpdated(task.TaskID)
		emailJSON(w, http.StatusOK, map[string]string{"project_key": task.ProjectKey})
	})
	mux.HandleFunc("POST /__fixture/stop", func(w http.ResponseWriter, r *http.Request) { once.Do(func() { close(stop) }); w.WriteHeader(204) })
	web := httptest.NewServer(mux)
	defer web.Close()
	manifest, _ := json.Marshal(map[string]any{"url": web.URL})
	if err := os.WriteFile("../../outputs/email-fixture.json", manifest, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("Browser fixture ready")
	select {
	case <-stop:
	case <-time.After(15 * time.Minute):
		t.Fatal("browser fixture timeout")
	}
}
