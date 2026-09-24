package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type policyTransport func(*http.Request) (*http.Response, error)

func (f policyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type policyFixture struct {
	p            mrPolicy
	state        string
	head         string
	reads        int
	closes       int
	notes        int
	mails        int
	addresses    []string
	pages        map[int][]map[string]string
	failPage     int
	changeHead   bool
	missingEmail bool
	mailFailure  bool
}

func newPolicyFixture(t *testing.T) *policyFixture {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := conn.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := conn.AutoMigrate(&db.MRPolicyRun{}, &userdb.User{}, &db.GitCommitLog{}, &db.TaskTelemetry{}, &db.Notification{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.GitLab.BaseURL = "https://gitlab.example.test"
	cfg.GitLab.APIToken = "test-secret"
	cfg.SMTP.Enabled = true
	f := &policyFixture{state: "opened", head: "sha1", pages: map[int][]map[string]string{1: {{"id": "sha1", "message": "fix sorting"}}}}
	f.p = mrPolicy{conn: conn, cfg: cfg, client: &http.Client{Transport: policyTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-secret" {
			t.Fatal("missing API authentication")
		}
		var body any
		headers := make(http.Header)
		status := 200
		switch {
		case strings.Contains(r.URL.Path, "/users/"):
			email := "author@example.test"
			if strings.HasSuffix(r.URL.Path, "/2") {
				email = "assign@example.test"
			}
			if f.missingEmail {
				email = ""
			}
			id := 1
			if strings.HasSuffix(r.URL.Path, "/2") {
				id = 2
			}
			body = policyUser{ID: id, Email: email}
		case strings.HasSuffix(r.URL.Path, "/commits"):
			page := 1
			fmt.Sscan(r.URL.Query().Get("page"), &page)
			if f.failPage == page {
				status = 503
				body = map[string]string{"message": "secret"}
			} else {
				body = f.pages[page]
				if _, ok := f.pages[page+1]; ok {
					headers.Set("X-Next-Page", fmt.Sprint(page+1))
				}
			}
		case strings.HasSuffix(r.URL.Path, "/notes"):
			f.notes++
			var note map[string]string
			json.NewDecoder(r.Body).Decode(&note)
			if note["body"] != missingJiraReason {
				t.Fatal("missing closure reason")
			}
			body = map[string]int{"id": 1}
		case r.Method == http.MethodPut:
			var change map[string]string
			json.NewDecoder(r.Body).Decode(&change)
			if change["state_event"] != "close" {
				t.Fatal("unexpected mutation")
			}
			f.closes++
			f.state = "closed"
			body = policyMR{State: "closed"}
		case r.Method == http.MethodGet:
			f.reads++
			sha := f.head
			if f.changeHead && f.reads > 1 {
				sha = "new-sha"
			}
			body = policyMR{State: f.state, SHA: sha, UpdatedAt: "2026-09-20T01:00:00Z", Title: "Improve ordering", WebURL: "https://gitlab.example.test/mr/7", Author: policyUser{ID: 1}, Assignees: []policyUser{{ID: 2}, {ID: 1}}}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		data, _ := json.Marshal(body)
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}, send: func(_ context.Context, _ config.SMTPConfig, to []string, subject, text, html string) error {
		f.mails++
		f.addresses = to
		if !strings.Contains(text, missingJiraReason) || !strings.Contains(subject, "缺少 Jira 编号") {
			t.Fatal("notification lacks reason")
		}
		if f.mailFailure {
			return errors.New("secret SMTP reply")
		}
		return nil
	}}
	return f
}
func TestMRPolicyAutoCloseIsPaused(t *testing.T) {
	f := newPolicyFixture(t)
	f.p.cfg.GitLab.Enabled = true
	f.p.cfg.GitLab.Repos = []config.RepoMapping{{Name: "repo", ProjectID: "10"}}
	oldDB, oldTransport := db.DB, http.DefaultTransport
	db.DB = f.p.conn
	requests := 0
	http.DefaultTransport = policyTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		return f.p.client.Transport.RoundTrip(r)
	})
	t.Cleanup(func() { db.DB, http.DefaultTransport = oldDB, oldTransport })
	for _, action := range []string{"open", "reopen", "update"} {
		payload := []byte(fmt.Sprintf(`{"project":{"id":10,"name":"repo"},"object_attributes":{"action":%q,"state":"opened","iid":7,"title":"Improve sorting","source_branch":"feature/sorting"}}`, action))
		if err := ProcessWebhookEvent(f.p.cfg, "Merge Request Hook", payload); err != nil {
			t.Fatal(err)
		}
	}
	var runs int64
	if err := f.p.conn.Model(&db.MRPolicyRun{}).Count(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if requests != 0 || runs != 0 || f.state != "opened" {
		t.Fatalf("paused policy caused side effects: requests=%d runs=%d state=%s", requests, runs, f.state)
	}
}

func TestMRPolicyReasonDoesNotIncludeExampleIssue(t *testing.T) {
	if jiraCommitKey.MatchString(missingJiraReason) {
		t.Fatal("MR policy reason must not include a concrete example Jira issue")
	}
}

func TestMRPolicyClosesAndNotifiesExactlyOnce(t *testing.T) {
	f := newPolicyFixture(t)
	for i := 0; i < 2; i++ {
		_, err := f.p.enforce(context.Background(), "10", 7)
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.closes != 1 || f.notes != 1 || f.mails != 1 || len(f.addresses) != 2 {
		t.Fatalf("close=%d notes=%d mails=%d recipients=%v", f.closes, f.notes, f.mails, f.addresses)
	}
	var run db.MRPolicyRun
	f.p.conn.First(&run)
	if run.Status != "sent" || run.Reason != missingJiraReason {
		t.Fatalf("incorrect decision ledger: %+v", run)
	}
}
func TestMRPolicyChecksAllCommitPages(t *testing.T) {
	f := newPolicyFixture(t)
	f.pages[2] = []map[string]string{{"id": "sha2", "message": "fix HR-4090 point sorting"}}
	closed, err := f.p.enforce(context.Background(), "10", 7)
	if err != nil || closed || f.closes != 0 || f.mails != 0 {
		t.Fatalf("valid Jira commit should allow MR: closed=%v err=%v", closed, err)
	}
}
func TestMRPolicyDoesNotCloseOnIncompleteOrStaleEvidence(t *testing.T) {
	for _, name := range []string{"page_error", "head_changed", "empty", "merged"} {
		t.Run(name, func(t *testing.T) {
			f := newPolicyFixture(t)
			switch name {
			case "page_error":
				f.failPage = 1
			case "head_changed":
				f.changeHead = true
			case "empty":
				f.pages[1] = nil
			case "merged":
				f.state = "merged"
			}
			_, err := f.p.enforce(context.Background(), "10", 7)
			if name != "merged" && err == nil {
				t.Fatal("expected diagnostic failure")
			}
			if f.closes != 0 || f.notes != 0 || f.mails != 0 {
				t.Fatal("unsafe side effects")
			}
		})
	}
}
func TestMRPolicyRecordsNotificationFailuresWithoutResending(t *testing.T) {
	for _, name := range []string{"missing_email", "smtp_failure", "smtp_disabled"} {
		t.Run(name, func(t *testing.T) {
			f := newPolicyFixture(t)
			switch name {
			case "missing_email":
				f.missingEmail = true
			case "smtp_failure":
				f.mailFailure = true
			case "smtp_disabled":
				f.p.cfg.SMTP.Enabled = false
			}
			closed, err := f.p.enforce(context.Background(), "10", 7)
			if !closed || err == nil {
				t.Fatal("closure and notification outcome must remain separate")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("credentials leaked")
			}
			_, _ = f.p.enforce(context.Background(), "10", 7)
			if f.closes != 1 || f.mails > 1 {
				t.Fatal("repeated destructive or uncertain delivery attempt")
			}
			var run db.MRPolicyRun
			f.p.conn.First(&run)
			expected := "notification_blocked"
			if name == "smtp_failure" {
				expected = "mail_failed_or_unknown"
			}
			if run.Status != expected {
				t.Fatalf("status=%s want=%s", run.Status, expected)
			}
		})
	}
}
func TestMergeWithoutTaskIDIsRecordedSeparately(t *testing.T) {
	f := newPolicyFixture(t)
	old := db.DB
	db.DB = f.p.conn
	t.Cleanup(func() { db.DB = old })
	payload := []byte(`{"project":{"name":"repo"},"object_attributes":{"action":"merge","state":"merged","iid":7,"title":"Improve sorting","source_branch":"feature/sorting"}}`)
	if err := ProcessWebhookEvent(&config.Config{}, "Merge Request Hook", payload); err != nil {
		t.Fatal(err)
	}
	var rows []db.GitCommitLog
	f.p.conn.Find(&rows)
	if len(rows) != 1 || rows[0].Action != "mr_merge" || rows[0].TaskID != "" || rows[0].MrIID != 7 {
		t.Fatalf("merge event missing or classified as commit: %+v", rows)
	}
}

func TestMRPolicyDurableClaimRejectsDuplicateSnapshot(t *testing.T) {
	f := newPolicyFixture(t)
	if _, err := f.p.enforce(context.Background(), "10", 7); err != nil {
		t.Fatal(err)
	}
	// A delayed duplicate may still see the old opened snapshot.
	f.state = "opened"
	if _, err := f.p.enforce(context.Background(), "10", 7); err != nil {
		t.Fatal(err)
	}
	if f.closes != 1 || f.mails != 1 {
		t.Fatal("duplicate snapshot repeated closure or mail")
	}
}

func TestJiraCommitKeywordSyntax(t *testing.T) {
	for _, message := range []string{"fix HR-4090 sorting", "HR-4090: repair", "refs CORE_API-123"} {
		if !jiraCommitKey.MatchString(message) {
			t.Fatalf("missed key in %q", message)
		}
	}
	for _, message := range []string{"fix sorting", "HR-", "HR-0", "hr-4090", "sha abc4090", "prefixHR-4090suffix"} {
		if jiraCommitKey.MatchString(message) {
			t.Fatalf("accepted invalid keyword %q", message)
		}
	}
}

func TestMRPolicyEmptyCommitMessageHasNoJiraKey(t *testing.T) {
	f := newPolicyFixture(t)
	f.pages[1][0]["message"] = ""
	closed, err := f.p.enforce(context.Background(), "10", 7)
	if !closed || err != nil {
		t.Fatalf("empty commit message is missing Jira key: closed=%v err=%v", closed, err)
	}
}
func TestMRPolicyMissingCommitMessageIsIncompleteEvidence(t *testing.T) {
	f := newPolicyFixture(t)
	delete(f.pages[1][0], "message")
	_, err := f.p.enforce(context.Background(), "10", 7)
	if err == nil || f.closes != 0 {
		t.Fatal("malformed API response must not cause closure")
	}
}
