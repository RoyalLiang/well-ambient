package readmodel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func validPageContract() Contract {
	return Contract{
		ID: "work-items", Class: ClassPage, TargetStrategy: StrategyKeyset,
		Maturity: MaturityVerified, Surfaces: []string{"tasks.status"},
		DefaultRows: 50, MaxRows: 100, MaxResponseBytes: 1 << 20,
		TargetQueryP95MS: 20, TargetHandlerP95MS: 50,
		Snapshot: "cursor carries scope fingerprint and high watermark",
	}
}

func TestRegistryAttributesGORMQueriesAndBudgetBreachesToTheReadContract(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file:read_contract_gorm_metrics?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	type metricRow struct {
		ID uint `gorm:"primaryKey"`
	}
	if err := conn.AutoMigrate(&metricRow{}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&[]metricRow{{}, {}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := InstallGORMObserver(conn); err != nil {
		t.Fatal(err)
	}
	contract := validPageContract()
	contract.TargetHandlerP95MS = 1
	contract.MaxResponseBytes = 4
	registry, err := NewRegistry(map[string]Contract{"/api/work-items": contract})
	if err != nil {
		t.Fatal(err)
	}
	handler := registry.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rows []metricRow
		if err := conn.WithContext(r.Context()).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		var scanned []metricRow
		if err := conn.WithContext(r.Context()).Table("metric_rows").Select("id").Scan(&scanned).Error; err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/work-items", nil))
	observations := registry.Snapshot()
	if len(observations) != 1 {
		t.Fatalf("observations = %+v", observations)
	}
	observation := observations[0]
	if observation.TotalQueries != 2 || observation.MaxQueries != 2 || observation.MaxRows < 1 {
		t.Fatalf("query attribution = %+v", observation)
	}
	if observation.QueryP95 <= 0 || observation.QueryMax <= 0 {
		t.Fatalf("query duration was not observed: %+v", observation)
	}
	if observation.HandlerBreaches != 1 || observation.ResponseBreaches != 1 {
		t.Fatalf("budget breaches = %+v", observation)
	}
}

func TestContractRejectsUnboundedPage(t *testing.T) {
	contract := validPageContract()
	contract.MaxRows = 201
	if err := contract.Validate(); err == nil {
		t.Fatal("expected a page above 200 rows to be rejected")
	}
}

func TestRegistryMatchesParameterizedRouteAndRecordsBoundedMetrics(t *testing.T) {
	contract := validPageContract()
	registry, err := NewRegistry(map[string]Contract{"/api/work-items/{id}": contract})
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	handler := registry.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/work-items/HIT-1", nil))
	if recorder.Header().Get("X-Well-Ambient-Read-Contract") != contract.ID {
		t.Fatalf("contract header = %q", recorder.Header().Get("X-Well-Ambient-Read-Contract"))
	}
	observations := registry.Snapshot()
	if len(observations) != 1 || observations[0].Requests != 1 || observations[0].LastStatus != http.StatusAccepted {
		t.Fatalf("unexpected observations: %+v", observations)
	}
	if observations[0].MaxBytes != int64(len(`{"ok":true}`)) {
		t.Fatalf("max bytes = %d", observations[0].MaxBytes)
	}
	if _, err := json.Marshal(observations); err != nil {
		t.Fatalf("marshal observations: %v", err)
	}
}
