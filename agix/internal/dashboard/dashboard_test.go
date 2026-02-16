package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/store"
)

func TestDashboardAPIStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{Budgets: map[string]config.Budget{}}
	d := New(cfg, st)

	mux := http.NewServeMux()
	d.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("stats status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to parse stats: %v", err)
	}
	if _, ok := stats["total_requests"]; !ok {
		t.Error("expected total_requests field in response")
	}
}

func TestDashboardAPIAgents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{Budgets: map[string]config.Budget{}}
	d := New(cfg, st)

	mux := http.NewServeMux()
	d.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("agents status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDashboardStaticFiles(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{Budgets: map[string]config.Budget{}}
	d := New(cfg, st)

	mux := http.NewServeMux()
	d.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/style.css", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("dashboard CSS status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if len(body) < 50 {
		t.Error("dashboard CSS body is too short")
	}
}
