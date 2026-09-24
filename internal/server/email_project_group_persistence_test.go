package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEmailProjectGroupsPersistAcrossSaveReadAndBootstrap(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(&db.ConfigVersion{}, &db.RuntimeConfig{}); err != nil {
		t.Fatal(err)
	}
	previousDB := db.DB
	db.DB = conn
	defer func() { db.DB = previousDB }()

	groups := []config.DailyJiraProjectGroup{
		{Name: "FMS 核心交付", Projects: []string{"FMS", "GPP"}, Owners: []string{"张三", "李四"}},
		{Name: "海外项目", Projects: []string{"FEL2WD"}, Owners: []string{"王五"}},
	}
	current := config.Config{}
	next := config.Config{DailyJiraEmail: config.DailyJiraEmailConfig{Timezone: "Asia/Shanghai", SendTime: "09:00", ProjectGroups: groups}}
	body, _ := json.Marshal(next)
	s := &Server{config: &current}
	recorder := httptest.NewRecorder()
	s.handleSaveConfig(recorder, httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !reflect.DeepEqual(current.DailyJiraEmail.ProjectGroups, groups) {
		t.Fatalf("runtime groups=%#v", current.DailyJiraEmail.ProjectGroups)
	}

	read := httptest.NewRecorder()
	s.handleGetConfig(read, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var response config.Config
	if err := json.Unmarshal(read.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.DailyJiraEmail.ProjectGroups, groups) {
		t.Fatalf("GET groups=%#v", response.DailyJiraEmail.ProjectGroups)
	}

	restarted := config.Config{}
	if err := BootstrapVersionedConfig(&restarted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restarted.DailyJiraEmail.ProjectGroups, groups) {
		t.Fatalf("bootstrap groups=%#v", restarted.DailyJiraEmail.ProjectGroups)
	}
}
