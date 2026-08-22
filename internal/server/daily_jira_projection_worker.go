package server

import (
	"context"
	"log"
	"time"

	"well-ambient/internal/dailyjira"
	"well-ambient/internal/db"
)

const (
	dailyJiraRolloverBatchSize = 5000
	dailyJiraRolloverInterval  = time.Minute
)

func (s *Server) startDailyJiraProjectionWorker(ctx context.Context) {
	if db.DB == nil {
		return
	}
	run := func() {
		for {
			result, err := dailyjira.RollForwardBatch(ctx, db.DB, time.Now(), dailyJiraRolloverBatchSize)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("Daily Jira projection rollover failed: %v", err)
				}
				return
			}
			if result.Done || result.Processed == 0 {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	run()
	ticker := time.NewTicker(dailyJiraRolloverInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
