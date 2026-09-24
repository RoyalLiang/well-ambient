package server

import (
	"context"
	"errors"
	"log"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

// Wall-clock comparison catches skipped DST minutes; durable dates prevent repeats.
func emailScheduleDue(now time.Time, cfg config.Config) (string, bool) {
	settings := cfg.DailyJiraEmail.Normalized()
	deliveryConfig := cfg
	deliveryConfig.DailyJiraEmail.Confluence.Enabled = false
	if !settings.Enabled || !cfg.SMTP.Enabled || config.ValidateMailSettings(deliveryConfig) != nil {
		return "", false
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return "", false
	}
	local := now.In(location)
	return local.Format("2006-01-02"), local.Format("15:04") >= settings.SendTime
}

func (s *Server) runEmailSchedule(ctx context.Context, now time.Time) {
	cfg := s.currentEmailConfig()
	date, due := emailScheduleDue(now, cfg)
	if !due || db.DB == nil {
		return
	}
	settings := cfg.DailyJiraEmail.Normalized()
	runContext, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var run db.DailyJiraEmailRun
	err := db.DB.WithContext(runContext).Where("date = ?", date).First(&run).Error
	if err == nil {
		// A date is claimed once, regardless of trigger or schedule edits.
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Daily Jira email: delivery ledger unavailable")
		return
	}

	_, err = s.sendDailyEmail(runContext, cfg, settings, date, "scheduled", now)
	if err != nil && !errors.Is(err, errEmailAlreadyClaimed) && ctx.Err() == nil {
		log.Printf("Daily Jira email attempt failed; inspect configuration and delivery ledger")
	}
}

func (s *Server) triggerEmailWorker() {
	if s == nil {
		return
	}
	s.emailConfigMu.Lock()
	ch := s.emailWorkerWakeup
	if ch == nil {
		ch = make(chan struct{}, 1)
		s.emailWorkerWakeup = ch
	}
	s.emailConfigMu.Unlock()
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *Server) startEmailWorker(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		// Catch up today's report after restart, without replaying past days.
		if ctx.Err() == nil {
			s.runEmailSchedule(ctx, time.Now())
		}
		s.emailConfigMu.Lock()
		wakeup := s.emailWorkerWakeup
		if wakeup == nil {
			wakeup = make(chan struct{}, 1)
			s.emailWorkerWakeup = wakeup
		}
		s.emailConfigMu.Unlock()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wakeup:
				s.runEmailSchedule(ctx, time.Now())
			case now := <-ticker.C:
				s.runEmailSchedule(ctx, now)
			}
		}
	}()
	return done
}
