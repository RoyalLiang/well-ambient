package readmodel

import (
	"time"

	"gorm.io/gorm"
)

const gormQueryStartedAt = "well_ambient:read_query_started_at"

// InstallGORMObserver connects SQL work to the read contract of the current
// HTTP request. Queries outside a wrapped request remain untouched. Handlers
// must use WithContext(r.Context()) for their SQL to be attributed; missing
// attribution is visible as zero query counts and is therefore auditable.
func InstallGORMObserver(conn *gorm.DB) error {
	if conn == nil {
		return gorm.ErrInvalidDB
	}
	start := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || metricsFromContext(tx.Statement.Context) == nil {
			return
		}
		tx.InstanceSet(gormQueryStartedAt, time.Now())
	}
	finish := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		metrics := metricsFromContext(tx.Statement.Context)
		if metrics == nil {
			return
		}
		started, ok := tx.InstanceGet(gormQueryStartedAt)
		if !ok {
			return
		}
		startedAt, ok := started.(time.Time)
		if !ok {
			return
		}
		metrics.record(time.Since(startedAt), tx.RowsAffected, tx.Error)
	}
	registrations := []func() error{
		func() error {
			return conn.Callback().Query().Before("gorm:query").Register("well_ambient:read_query_start", start)
		},
		func() error {
			return conn.Callback().Query().After("gorm:query").Register("well_ambient:read_query_finish", finish)
		},
		func() error {
			return conn.Callback().Row().Before("gorm:row").Register("well_ambient:read_row_start", start)
		},
		func() error {
			return conn.Callback().Row().After("gorm:row").Register("well_ambient:read_row_finish", finish)
		},
		func() error {
			return conn.Callback().Raw().Before("gorm:raw").Register("well_ambient:read_raw_start", start)
		},
		func() error {
			return conn.Callback().Raw().After("gorm:raw").Register("well_ambient:read_raw_finish", finish)
		},
	}
	for _, register := range registrations {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}
