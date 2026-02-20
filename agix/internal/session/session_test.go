package session

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/agent-platform/agix/internal/store"
	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func float64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int             { return &v }

func TestSetAndGet(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	tests := []struct {
		name     string
		override Override
	}{
		{
			name: "model only",
			override: Override{
				SessionID: "s1",
				AgentName: "agent-a",
				Model:     "gpt-4o-mini",
			},
		},
		{
			name: "temperature only",
			override: Override{
				SessionID:   "s2",
				AgentName:   "agent-b",
				Temperature: float64Ptr(0.5),
			},
		},
		{
			name: "all fields",
			override: Override{
				SessionID:   "s3",
				AgentName:   "agent-c",
				Model:       "claude-sonnet-4-20250514",
				Temperature: float64Ptr(0.8),
				MaxTokens:   intPtr(2048),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := mgr.Set(&tc.override); err != nil {
				t.Fatalf("Set: %v", err)
			}

			got, err := mgr.Get(tc.override.SessionID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got == nil {
				t.Fatal("Get returned nil")
			}
			if got.SessionID != tc.override.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tc.override.SessionID)
			}
			if got.AgentName != tc.override.AgentName {
				t.Errorf("AgentName = %q, want %q", got.AgentName, tc.override.AgentName)
			}
			if got.Model != tc.override.Model {
				t.Errorf("Model = %q, want %q", got.Model, tc.override.Model)
			}
			if tc.override.Temperature != nil {
				if got.Temperature == nil || *got.Temperature != *tc.override.Temperature {
					t.Errorf("Temperature mismatch")
				}
			}
			if tc.override.MaxTokens != nil {
				if got.MaxTokens == nil || *got.MaxTokens != *tc.override.MaxTokens {
					t.Errorf("MaxTokens mismatch")
				}
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	got, err := mgr.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestExpiry(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	// Set with already-expired time
	o := &Override{
		SessionID: "expired-1",
		AgentName: "agent-x",
		Model:     "gpt-4o",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := mgr.Set(o); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := mgr.Get("expired-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for expired session, got %+v", got)
	}
}

func TestCleanExpired(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	// Insert one expired and one active
	expired := &Override{
		SessionID: "exp-1",
		AgentName: "a",
		Model:     "gpt-4o",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	active := &Override{
		SessionID: "act-1",
		AgentName: "b",
		Model:     "gpt-4o",
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}
	if err := mgr.Set(expired); err != nil {
		t.Fatalf("Set expired: %v", err)
	}
	if err := mgr.Set(active); err != nil {
		t.Fatalf("Set active: %v", err)
	}

	n, err := mgr.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("CleanExpired deleted %d, want 1", n)
	}

	// Active should still exist
	got, err := mgr.Get("act-1")
	if err != nil {
		t.Fatalf("Get active: %v", err)
	}
	if got == nil {
		t.Error("active session was cleaned")
	}
}

func TestDelete(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	o := &Override{SessionID: "del-1", AgentName: "a", Model: "gpt-4o"}
	if err := mgr.Set(o); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := mgr.Delete("del-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := mgr.Get("del-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestApplyOverrides(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		override Override
		wantModel string
		wantTemp  *float64
		wantMax   *int
	}{
		{
			name:      "override model",
			body:      `{"model":"gpt-4o","messages":[]}`,
			override:  Override{Model: "gpt-4o-mini"},
			wantModel: "gpt-4o-mini",
		},
		{
			name:      "override temperature",
			body:      `{"model":"gpt-4o","messages":[],"temperature":1.0}`,
			override:  Override{Temperature: float64Ptr(0.3)},
			wantModel: "gpt-4o",
			wantTemp:  float64Ptr(0.3),
		},
		{
			name:      "override max_tokens",
			body:      `{"model":"gpt-4o","messages":[]}`,
			override:  Override{MaxTokens: intPtr(512)},
			wantModel: "gpt-4o",
			wantMax:   intPtr(512),
		},
		{
			name:      "override all",
			body:      `{"model":"gpt-4o","messages":[],"temperature":1.0,"max_tokens":4096}`,
			override:  Override{Model: "claude-sonnet-4-20250514", Temperature: float64Ptr(0.2), MaxTokens: intPtr(1024)},
			wantModel: "claude-sonnet-4-20250514",
			wantTemp:  float64Ptr(0.2),
			wantMax:   intPtr(1024),
		},
		{
			name:      "no overrides",
			body:      `{"model":"gpt-4o","messages":[]}`,
			override:  Override{},
			wantModel: "gpt-4o",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Apply([]byte(tc.body), &tc.override)

			var parsed map[string]json.RawMessage
			if err := json.Unmarshal(result, &parsed); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}

			var model string
			json.Unmarshal(parsed["model"], &model)
			if model != tc.wantModel {
				t.Errorf("model = %q, want %q", model, tc.wantModel)
			}

			if tc.wantTemp != nil {
				var temp float64
				json.Unmarshal(parsed["temperature"], &temp)
				if temp != *tc.wantTemp {
					t.Errorf("temperature = %v, want %v", temp, *tc.wantTemp)
				}
			}

			if tc.wantMax != nil {
				var max int
				json.Unmarshal(parsed["max_tokens"], &max)
				if max != *tc.wantMax {
					t.Errorf("max_tokens = %d, want %d", max, *tc.wantMax)
				}
			}
		})
	}
}

func TestListActive(t *testing.T) {
	db := testDB(t)
	mgr, err := New(db, time.Hour, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	// Insert two active, one expired
	for _, o := range []*Override{
		{SessionID: "a1", AgentName: "x", Model: "gpt-4o"},
		{SessionID: "a2", AgentName: "y", Model: "gpt-4o-mini"},
		{SessionID: "e1", AgentName: "z", Model: "gpt-4o", ExpiresAt: time.Now().UTC().Add(-time.Hour)},
	} {
		if err := mgr.Set(o); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	list, err := mgr.ListActive()
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListActive returned %d, want 2", len(list))
	}
}
