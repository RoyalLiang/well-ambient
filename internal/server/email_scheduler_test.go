package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestEmailScheduleDueOnlyWithinConfiguredMinute(t *testing.T) {
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.SendTime = "09:30"
	for _, tc := range []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before", time.Date(2026, 10, 3, 1, 29, 59, 0, time.UTC), false},
		{"first_second", time.Date(2026, 10, 3, 1, 30, 0, 0, time.UTC), true},
		{"last_tick", time.Date(2026, 10, 3, 1, 30, 59, 999999999, time.UTC), true},
		{"following_minute", time.Date(2026, 10, 3, 1, 31, 0, 0, time.UTC), false},
		{"noon", time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date, due := emailScheduleDue(tc.now, cfg)
			if due != tc.want || date != "2026-10-03" {
				t.Fatalf("date=%q due=%t, want date=2026-10-03 due=%t", date, due, tc.want)
			}
		})
	}
}

func TestEmailSchedulerStartupAfterSendMinuteDoesNotCatchUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		now  time.Time
	}{
		{"following_minute", time.Date(2026, 10, 3, 1, 31, 0, 0, time.UTC)},
		{"noon", time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := emailTestDatabase(t)
			cfg := emailTestConfig()
			cfg.DailyJiraEmail.Enabled = true
			cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
			cfg.DailyJiraEmail.SendTime = "09:30"
			var calls atomic.Int32
			s := &Server{config: &cfg}
			s.setEmailConfig(cfg)
			s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
				calls.Add(1)
				return nil
			}

			// This is the same schedule check used at worker startup, with a fresh
			// server and no durable claim for today's date.
			s.runEmailSchedule(context.Background(), tc.now)
			if got := calls.Load(); got != 0 {
				t.Errorf("late startup sent %d emails; want no automatic catch-up", got)
			}
			var count int64
			if err := conn.Model(&db.DailyJiraEmailRun{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("late startup created %d delivery claims; want none", count)
			}
		})
	}
}

func TestEmailSchedulerWithinMinuteClaimsOnceAndSendsNextDay(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.SendTime = "09:30"
	var calls atomic.Int32
	sender := func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	s := &Server{config: &cfg, emailSender: sender}
	s.setEmailConfig(cfg)
	now := time.Date(2026, 10, 3, 1, 30, 49, 0, time.UTC)
	s.runEmailSchedule(context.Background(), now)
	if got := calls.Load(); got != 1 {
		t.Fatalf("scheduled-minute sends=%d, want 1", got)
	}
	s.runEmailSchedule(context.Background(), now.Add(10*time.Second))
	restarted := &Server{config: &cfg, emailSender: sender}
	restarted.setEmailConfig(cfg)
	restarted.runEmailSchedule(context.Background(), now.Add(10*time.Second))
	restarted.runEmailSchedule(context.Background(), now.Add(time.Minute))
	if got := calls.Load(); got != 1 {
		t.Fatalf("same-day repeated checks or restart sent %d emails, want 1", got)
	}

	// Crossing the calendar date allows one new scheduled attempt.
	restarted.runEmailSchedule(context.Background(), now.AddDate(0, 0, 1))
	if got := calls.Load(); got != 2 {
		t.Fatalf("next-day total sends=%d, want 2", got)
	}
	var runs []db.DailyJiraEmailRun
	if err := conn.Order("date").Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("delivery claims=%d, want 2", len(runs))
	}
	for i, wantDate := range []string{"2026-10-03", "2026-10-04"} {
		if runs[i].Date != wantDate || runs[i].Status != "sent" || runs[i].Trigger != "scheduled" {
			t.Fatalf("unexpected scheduled delivery: %+v", runs[i])
		}
	}
}

func TestEmailManualSendStillAvailableAfterMissedSchedule(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.SendTime = "09:30"
	var calls atomic.Int32
	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	now := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	s.runEmailSchedule(context.Background(), now)
	if got := calls.Load(); got != 0 {
		t.Fatalf("missed schedule sent %d emails before manual request, want 0", got)
	}
	if _, err := s.sendDailyEmail(context.Background(), cfg, cfg.DailyJiraEmail, "2026-10-03", "manual", now); err != nil {
		t.Fatalf("manual send after missed schedule failed: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("manual transport calls=%d, want 1", got)
	}
	var run db.DailyJiraEmailRun
	if err := conn.First(&run, "date = ?", "2026-10-03").Error; err != nil {
		t.Fatal(err)
	}
	if run.Trigger != "manual" || run.Status != "sent" {
		t.Fatalf("unexpected manual delivery claim: %+v", run)
	}
}

func TestEmailSchedulerSpringForwardSkipsNonexistentMinute(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "America/New_York"
	cfg.DailyJiraEmail.SendTime = "02:30"
	var calls atomic.Int32
	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	// On March 8, the local clock jumps from 01:59 to 03:00; 02:30
	// never occurs and must not become an automatic 03:00 catch-up.
	for _, now := range []time.Time{
		time.Date(2026, 3, 8, 6, 59, 0, 0, time.UTC),
		time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC),
	} {
		s.runEmailSchedule(context.Background(), now)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("nonexistent local minute sent %d emails, want 0", got)
	}
	var count int64
	if err := conn.Model(&db.DailyJiraEmailRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("nonexistent local minute created %d delivery claims, want 0", count)
	}
}

func TestEmailSchedulerFallBackRepeatedMinuteClaimsOnce(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "America/New_York"
	cfg.DailyJiraEmail.SendTime = "01:30"
	var calls atomic.Int32
	sender := func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	// Both UTC times represent 01:30 on November 1, on opposite sides
	// of the DST offset change. A new server must respect the first claim.
	for _, hour := range []int{5, 6} {
		s := &Server{config: &cfg, emailSender: sender}
		s.setEmailConfig(cfg)
		s.runEmailSchedule(context.Background(), time.Date(2026, 11, 1, hour, 30, 0, 0, time.UTC))
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("repeated local minute sent %d emails, want 1", got)
	}
	var runs []db.DailyJiraEmailRun
	if err := conn.Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Date != "2026-11-01" || runs[0].Status != "sent" {
		t.Fatalf("unexpected repeated-minute delivery claims: %+v", runs)
	}
}

func TestEmailSchedulerConfigChangeDoesNotCatchUpMissedMinute(t *testing.T) {
	conn := emailTestDatabase(t)
	cfg := emailTestConfig()
	cfg.DailyJiraEmail.Enabled = true
	cfg.DailyJiraEmail.Timezone = "Asia/Shanghai"
	cfg.DailyJiraEmail.SendTime = "10:00"
	var calls atomic.Int32
	s := &Server{config: &cfg}
	s.setEmailConfig(cfg)
	s.emailSender = func(context.Context, config.SMTPConfig, []string, string, string, string) error {
		calls.Add(1)
		return nil
	}
	now := time.Date(2026, 10, 3, 1, 31, 0, 0, time.UTC)
	s.runEmailSchedule(context.Background(), now)
	// Saving configuration wakes the worker and runs this same check.
	// A newly selected time in the past must not claim today's delivery.
	cfg.DailyJiraEmail.SendTime = "09:30"
	s.setEmailConfig(cfg)
	s.runEmailSchedule(context.Background(), now)
	if got := calls.Load(); got != 0 {
		t.Fatalf("config edit to missed minute sent %d emails, want 0", got)
	}
	var count int64
	if err := conn.Model(&db.DailyJiraEmailRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("config edit to missed minute created %d delivery claims, want 0", count)
	}

	cfg.DailyJiraEmail.SendTime = "09:31"
	s.setEmailConfig(cfg)
	s.runEmailSchedule(context.Background(), now)
	if got := calls.Load(); got != 1 {
		t.Fatalf("config edit to current minute sent %d emails, want 1", got)
	}
	if err := conn.Model(&db.DailyJiraEmailRun{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("current-minute delivery claims=%d, want 1", count)
	}
}
