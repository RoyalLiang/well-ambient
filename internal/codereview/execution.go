package codereview

import (
	"context"
	"time"

	"well-ambient/internal/db"
)

// Heartbeats distinguish a live single-worker execution from an interrupted
// process. They do not provide fencing between attempts in multiple workers.
func (s *Service) monitorReviewRun(ctx context.Context, cancel context.CancelFunc, id uint) {
	poll, heartbeat := s.reviewPollInterval, s.reviewHeartbeatInterval
	if poll <= 0 {
		poll = time.Second
	}
	if heartbeat <= 0 {
		heartbeat = 30 * time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	nextHeartbeat := time.Now().Add(heartbeat)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			var current db.CodeReviewRun
			result := s.DB.WithContext(ctx).Select("status").Where("id = ?", id).Find(&current)
			if result.Error != nil {
				continue
			}
			if current.Status != "running" {
				cancel()
				return
			}
			if now.Before(nextHeartbeat) {
				continue
			}
			// Never pass the shared run struct to GORM from this goroutine.
			result = s.DB.WithContext(ctx).Model(&db.CodeReviewRun{}).Where("id = ? AND status = ?", id, "running").UpdateColumn("updated_at", now)
			if result.Error != nil {
				continue
			}
			if result.RowsAffected == 0 {
				cancel()
				return
			}
			nextHeartbeat = now.Add(heartbeat)
		}
	}
}
