package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/confluence"
	"well-ambient/internal/db"
)

type confluenceDiagnosticTransport func(*http.Request) (*http.Response, error)

func (f confluenceDiagnosticTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEmailConfluenceFailurePreservesSafeReason(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	client := confluence.Client{HTTPClient: &http.Client{Transport: confluenceDiagnosticTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("private upstream body " + cfg.DailyJiraEmail.Confluence.Token)), Header: make(http.Header)}, nil
	})}}
	s := &Server{config: &cfg}
	s.emailConfluenceSync = func(ctx context.Context, _ config.ConfluenceSyncConfig, _ *emailReport) (string, error) {
		_, err := client.Check(ctx, confluence.Config{ParentPageURL: "https://confluence.example.test/pages/viewpage.action?pageId=1", Token: cfg.DailyJiraEmail.Confluence.Token})
		return "", err
	}
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		// SMTP must still run when Confluence is unavailable.
		return nil
	}
	_, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", time.Now())
	if err == nil || !strings.Contains(err.Error(), "Confluence 拒绝访问") {
		t.Fatalf("safe permission diagnosis was lost: %v", err)
	}
	if strings.Contains(err.Error(), cfg.DailyJiraEmail.Confluence.Token) || strings.Contains(err.Error(), "private upstream body") {
		t.Fatal("diagnosis leaked upstream secrets")
	}
	var run db.DailyJiraEmailRun
	if err := conn.First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "sent" || run.ConfluenceStatus != "failed" || run.RecipientsJSON == "" {
		t.Fatalf("unexpected ledger status %s", run.Status)
	}
}

func TestInvalidConfluenceConfigDoesNotBlockSMTP(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	cfg.DailyJiraEmail.Confluence.ParentPageURL = "invalid-parent"
	s := &Server{config: &cfg}
	sends := 0
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error { sends++; return nil }
	_, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", time.Now())
	var run db.DailyJiraEmailRun
	if e := conn.First(&run).Error; e != nil {
		t.Fatalf("missing delivery ledger; sends=%d error=%v", sends, err)
	}
	if sends != 1 || run.Status != "sent" || run.ConfluenceStatus != "failed" || err == nil {
		t.Fatalf("sends=%d status=%s sync=%s error=%v", sends, run.Status, run.ConfluenceStatus, err)
	}
}
