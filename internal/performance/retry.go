package performance

import (
	"context"
	"strings"
	"time"
)

func (m *Module) withBusyRetry(ctx context.Context, operation func() error) error {
	settings := m.currentSettings()
	for attempt := 1; ; attempt++ {
		err := operation()
		if err == nil || !isSQLiteBusy(err) || attempt >= settings.BusyRetries {
			return err
		}
		timer := time.NewTimer(settings.BusyRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "sqlite_busy") ||
		strings.Contains(message, "sqlite_locked")
}
