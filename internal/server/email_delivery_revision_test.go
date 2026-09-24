package server

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestSentEmailConcurrentRetryOnlySyncsConfluence(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := confluenceEmailTestConfig()
	conn.Create(&db.DailyJiraEmailRun{Date: "2026-03-09", Status: "sent", ConfluenceStatus: "failed", RecipientsJSON: `["original@example.test"]`})
	var calls atomic.Int32
	s := &Server{config: &cfg}
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		t.Error("SMTP must never repeat on document retry")
		return nil
	}
	s.emailConfluenceSync = func(context.Context, config.ConfluenceSyncConfig, *emailReport) (string, error) {
		calls.Add(1)
		return "https://confluence.example.test/page/1", nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", "manual", time.Now())
		}()
	}
	wg.Wait()
	var run db.DailyJiraEmailRun
	conn.First(&run)
	if calls.Load() != 1 || run.Status != "sent" || run.ConfluenceStatus != "synced" || run.RecipientsJSON != `["original@example.test"]` {
		t.Fatalf("incorrect retry result: calls=%d run=%+v", calls.Load(), run)
	}
}

func TestYesterdayCommitMRBarsDedupeAndTimeScope(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	today, yesterday, _, err := emailDateBounds("2026-03-09", cfg.DailyJiraEmail.Timezone, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	at := yesterday.Add(time.Hour)
	logs := []db.GitCommitLog{
		{Repo: "dispatch", Action: "git_push", CommitID: "abc", Author: "alice", CreatedAt: at},
		{Repo: "dispatch", Action: "git_push", CommitID: "abc", Author: "alice", CreatedAt: at},
		{Repo: "dispatch", Action: "mr_open", MrIID: 1, Author: "alice", CreatedAt: at},
		{Repo: "dispatch", Action: "mr_merge", MrIID: 1, Author: "alice", CreatedAt: at},
		{Repo: "vehicle", Action: "mr_open", MrIID: 1, Author: "alice", CreatedAt: at},
		{Repo: "dispatch", Action: "mr_open", MrIID: 2, Author: "alice", CreatedAt: today},
		{Repo: "dispatch", Action: "mr_open", MrIID: 3, Author: "outsider", CreatedAt: at},
	}
	conn.Create(&logs)
	s := &Server{config: &cfg}
	report, err := s.buildEmailReport(context.Background(), cfg, cfg.DailyJiraEmail, "2026-03-09", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := report.ActivityChart
	if c == nil || len(c.Bars) != 2 || c.Bars[0].Count != 1 || c.Bars[1].Count != 2 {
		t.Fatalf("wrong dedupe/time/identity counts: %+v", c)
	}
	if !strings.Contains(report.HTML, "昨日 Commit / MR") || !strings.Contains(report.HTML, "email-activity-bar") {
		t.Fatal("email lacks top activity bars")
	}
	storage, _, err := confluenceReportStorage(&report)
	if err != nil || !strings.Contains(storage, "昨日 Commit / MR") || !strings.Contains(storage, "MR · 2") {
		t.Fatalf("Confluence lacks counts: %v", err)
	}
}
