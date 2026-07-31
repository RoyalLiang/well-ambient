package db

import (
	"fmt"
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openProjectPreferenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&UserProjectPreference{}, &TaskTelemetry{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return conn
}

func TestReplaceUserProjectPreferencesUsesEmptyAsAllAndIsolatesUsers(t *testing.T) {
	conn := openProjectPreferenceTestDB(t)
	if err := ReplaceUserProjectPreferences(conn, "alice@example.com", []string{" ns2 ", "hit", "HIT"}); err != nil {
		t.Fatalf("save Alice preferences: %v", err)
	}
	if err := ReplaceUserProjectPreferences(conn, "bob@example.com", []string{"DG"}); err != nil {
		t.Fatalf("save Bob preferences: %v", err)
	}

	aliceKeys, err := LoadUserProjectPreferenceKeys(conn, "alice@example.com")
	if err != nil {
		t.Fatalf("load Alice preferences: %v", err)
	}
	if want := []string{"HIT", "NS2"}; !reflect.DeepEqual(aliceKeys, want) {
		t.Fatalf("Alice keys = %#v, want %#v", aliceKeys, want)
	}
	bobKeys, err := LoadUserProjectPreferenceKeys(conn, "bob@example.com")
	if err != nil {
		t.Fatalf("load Bob preferences: %v", err)
	}
	if want := []string{"DG"}; !reflect.DeepEqual(bobKeys, want) {
		t.Fatalf("Bob keys = %#v, want %#v", bobKeys, want)
	}

	if err := ReplaceUserProjectPreferences(conn, "alice@example.com", nil); err != nil {
		t.Fatalf("restore Alice all-project preference: %v", err)
	}
	aliceKeys, err = LoadUserProjectPreferenceKeys(conn, "alice@example.com")
	if err != nil {
		t.Fatalf("reload Alice preferences: %v", err)
	}
	if len(aliceKeys) != 0 {
		t.Fatalf("Alice keys after reset = %#v, want empty all-project sentinel", aliceKeys)
	}
}

func TestApplyTaskProjectScopeIntersectsTaskQuery(t *testing.T) {
	conn := openProjectPreferenceTestDB(t)
	tasks := []TaskTelemetry{
		{TaskID: "HIT-101", Title: "HIT demand"},
		{TaskID: "NS2-202", Title: "NS2 Jira"},
		{TaskID: "DG-303", Title: "DG Jira"},
		{TaskID: "LEGACY-404", ProjectKey: "HIT", Title: "explicit HIT"},
		{TaskID: "SYSTEM", Title: "unscoped"},
	}
	if err := conn.Create(&tasks).Error; err != nil {
		t.Fatalf("seed tasks: %v", err)
	}

	var scoped []TaskTelemetry
	if err := ApplyTaskProjectScope(conn.Model(&TaskTelemetry{}), []string{"hit", "NS2"}).Order("task_id asc").Find(&scoped).Error; err != nil {
		t.Fatalf("query scoped tasks: %v", err)
	}
	got := make([]string, 0, len(scoped))
	for _, task := range scoped {
		got = append(got, task.TaskID)
	}
	if want := []string{"HIT-101", "LEGACY-404", "NS2-202"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("scoped task IDs = %#v, want %#v", got, want)
	}

	var all []TaskTelemetry
	if err := ApplyTaskProjectScope(conn.Model(&TaskTelemetry{}), nil).Find(&all).Error; err != nil {
		t.Fatalf("query all tasks: %v", err)
	}
	if len(all) != len(tasks) {
		t.Fatalf("all-project query returned %d tasks, want %d", len(all), len(tasks))
	}
	if !TaskMatchesProjectScope("hit-999", []string{"HIT"}) {
		t.Fatal("case-insensitive task scope should match")
	}
	if TaskMatchesProjectScope("DG-999", []string{"HIT"}) {
		t.Fatal("unselected project should not match")
	}
	if !TaskMatchesExplicitProjectScope("HIT", "DG-999", []string{"HIT"}) {
		t.Fatal("explicit project must take precedence over a conflicting legacy prefix")
	}
	if TaskMatchesExplicitProjectScope("DG", "HIT-999", []string{"HIT"}) {
		t.Fatal("an explicit unselected project must not fall back to the task id")
	}
}

type projectPreferenceDemandRow struct {
	ID       uint `gorm:"primaryKey"`
	DemandID string
}

func (projectPreferenceDemandRow) TableName() string {
	return "project_preference_demand_rows"
}

func TestApplyDemandProjectScopeIntersectsDemandQuery(t *testing.T) {
	conn := openProjectPreferenceTestDB(t)
	if err := conn.AutoMigrate(&projectPreferenceDemandRow{}); err != nil {
		t.Fatalf("migrate demand rows: %v", err)
	}
	rows := []projectPreferenceDemandRow{{DemandID: "HIT-11"}, {DemandID: "NS2-12"}, {DemandID: "DG-13"}}
	if err := conn.Create(&rows).Error; err != nil {
		t.Fatalf("seed demand rows: %v", err)
	}

	var scoped []projectPreferenceDemandRow
	if err := ApplyDemandProjectScope(conn.Model(&projectPreferenceDemandRow{}), []string{"NS2"}).Find(&scoped).Error; err != nil {
		t.Fatalf("query scoped demand rows: %v", err)
	}
	if len(scoped) != 1 || scoped[0].DemandID != "NS2-12" {
		t.Fatalf("scoped demand rows = %#v, want only NS2-12", scoped)
	}
}
