package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDecisionTablePreferenceHandlerTest(t *testing.T) *Server {
	t.Helper()
	previousDB := db.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&db.UserTablePreference{}); err != nil {
		t.Fatalf("migrate preference: %v", err)
	}
	db.DB = conn
	t.Cleanup(func() { db.DB = previousDB })
	return NewServer(&config.Config{}, "")
}

func TestDecisionTablePreferenceHandlersDefaultSaveAndValidate(t *testing.T) {
	server := setupDecisionTablePreferenceHandlerTest(t)

	get := httptest.NewRequest(http.MethodGet, "/api/me/decision-table-columns", nil)
	get.Header.Set("x-authenticated-user-id", "alice@example.com")
	getRecorder := httptest.NewRecorder()
	server.handleGetDecisionTableColumns(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("default GET status = %d, body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	var initial decisionTableColumnPreferenceResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode default response: %v", err)
	}
	if strings.Join(initial.VisibleColumns, ",") != "task_id,title,owner,risk,due,status" {
		t.Fatalf("default columns = %#v", initial.VisibleColumns)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/me/decision-table-columns", strings.NewReader(`{"visible_columns":["task_id","title","status"]}`))
	put.Header.Set("x-authenticated-user-id", "alice@example.com")
	putRecorder := httptest.NewRecorder()
	server.handleUpdateDecisionTableColumns(putRecorder, put)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", putRecorder.Code, putRecorder.Body.String())
	}

	reload := httptest.NewRequest(http.MethodGet, "/api/me/decision-table-columns", nil)
	reload.Header.Set("x-authenticated-user-id", "alice@example.com")
	reloadRecorder := httptest.NewRecorder()
	server.handleGetDecisionTableColumns(reloadRecorder, reload)
	var saved decisionTableColumnPreferenceResponse
	if err := json.Unmarshal(reloadRecorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved response: %v", err)
	}
	if strings.Join(saved.VisibleColumns, ",") != "task_id,title,status" {
		t.Fatalf("saved columns = %#v", saved.VisibleColumns)
	}

	invalid := httptest.NewRequest(http.MethodPut, "/api/me/decision-table-columns", strings.NewReader(`{"visible_columns":["task_id","unknown"]}`))
	invalid.Header.Set("x-authenticated-user-id", "alice@example.com")
	invalidRecorder := httptest.NewRecorder()
	server.handleUpdateDecisionTableColumns(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid column status = %d, body=%s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func TestDecisionTablePreferenceRequiresTaskIDColumn(t *testing.T) {
	server := setupDecisionTablePreferenceHandlerTest(t)
	request := httptest.NewRequest(http.MethodPut, "/api/me/decision-table-columns", strings.NewReader(`{"visible_columns":["title","status"]}`))
	request.Header.Set("x-authenticated-user-id", "alice@example.com")
	recorder := httptest.NewRecorder()
	server.handleUpdateDecisionTableColumns(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing task_id status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}
