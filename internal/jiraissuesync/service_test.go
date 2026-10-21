package jiraissuesync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func fixture(t *testing.T) *Service {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.AutoMigrate(&db.JiraIssueSyncConfig{}, &db.JiraIssueSyncPreview{}, &db.TaskTelemetry{}); err != nil {
		t.Fatal(err)
	}
	sql, _ := conn.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	return &Service{DB: conn, Projects: []string{"NS2"}, Repos: []config.RepoMapping{{ProjectID: "42", Path: "ops/terminal"}, {ProjectID: "99", Path: "other/app"}}, Now: func() time.Time { return time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC) }}
}

func TestDailyPreviewScopeNoFanoutAndIdempotency(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	v, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Enabled || v.TimeOfDay != "09:00" {
		t.Fatal("unsafe defaults")
	}
	v.JiraProject = "NS2"
	v.GroupPath = "ops"
	v.TargetProjectID = "42"
	v.Enabled = true
	v, err = s.Save(ctx, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []db.TaskTelemetry{{TaskID: "NS2-1", ProjectKey: "NS2", Source: "jira", Title: "车辆反馈", Description: "需人工审阅"}, {TaskID: "OTHER-2", ProjectKey: "OTHER", Source: "jira"}, {TaskID: "LOCAL-1", ProjectKey: "NS2", Source: "manual"}} {
		if err = s.DB.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	var count int64
	s.DB.Model(&db.JiraIssueSyncPreview{}).Count(&count)
	if count != 1 {
		t.Fatalf("daily duplicated: %d", count)
	}
	p, err := s.Latest(ctx)
	if err != nil || p.Total != 1 || len(p.Items) != 1 || p.TargetProjectID != "42" || p.RemoteVerified || p.Mode != "preview_only" {
		t.Fatalf("bad preview: %+v %v", p, err)
	}
	// Pruning the daily preview must not erase the durable daily execution marker.
	for i := 0; i < 31; i++ {
		if _, err = s.Generate(ctx, v, fmt.Sprintf("manual-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	var before uint
	s.DB.Model(&db.JiraIssueSyncPreview{}).Select("MAX(id)").Scan(&before)
	if err = s.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	var after uint
	s.DB.Model(&db.JiraIssueSyncPreview{}).Select("MAX(id)").Scan(&after)
	if before != after {
		t.Fatal("daily preview repeated after pruning history")
	}

	v.TargetProjectID = "99"
	if _, err = s.Save(ctx, v); openaccess.ErrorCode(err) != "invalid_query" {
		t.Fatal("cross-group target accepted")
	}
	v.TargetProjectID = "42"
	v.Version--
	if _, err = s.Save(ctx, v); openaccess.ErrorCode(err) != "conflict" {
		t.Fatal("stale save accepted")
	}
}

func TestPreviewBoundAndDailyTime(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	v, _ := s.Settings(ctx)
	v.JiraProject = "NS2"
	v.GroupPath = "ops"
	v.TargetProjectID = "42"
	v.Enabled = true
	v.TimeOfDay = "23:00"
	v, err := s.Save(ctx, v)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 103; i++ {
		if err = s.DB.Create(&db.TaskTelemetry{TaskID: fmt.Sprintf("NS2-%03d", i), ProjectKey: "NS2", Source: "jira"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Latest(ctx)
	if p != nil {
		t.Fatal("ran before due time")
	}
	preview, err := s.Generate(ctx, v, "manual-1")
	if err != nil || preview.Total != 103 || len(preview.Items) != 100 || !preview.Truncated {
		t.Fatalf("unbounded preview: %+v %v", preview, err)
	}
}
