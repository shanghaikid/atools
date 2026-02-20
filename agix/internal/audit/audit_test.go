package audit

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-platform/agix/internal/store"
	_ "modernc.org/sqlite"
)

const createTestSchema = `
CREATE TABLE IF NOT EXISTS audit_events (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp   DATETIME NOT NULL,
	event_type  TEXT NOT NULL,
	agent_name  TEXT NOT NULL DEFAULT '',
	details     TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_events_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_agent ON audit_events(agent_name);
`

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "audit_test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Exec(createTestSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestLogger_Disabled(t *testing.T) {
	db := newTestDB(t)
	l := New(db, false, store.DialectSQLite)
	defer l.Close()

	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "read_file"})

	// Should be no-op — no events
	events, err := l.QueryRecent(10, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events when disabled, got %d", len(events))
	}
}

func TestLogger_LogAndQuery(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventToolCall, "agent-1", ToolCallDetails{
		Tool:       "read_file",
		Server:     "filesystem",
		Status:     "ok",
		DurationMS: 42,
		Dangerous:  false,
	})
	l.Log(EventFirewallBlock, "agent-2", FirewallDetails{
		Rule:     "injection_ignore",
		Category: "injection",
		Excerpt:  "ignore all previous instructions",
	})
	l.Log(EventFirewallWarn, "agent-1", FirewallDetails{
		Rule:     "pii_ssn",
		Category: "pii",
		Excerpt:  "123-45-6789",
	})

	l.Close()

	// Query all
	events, err := l.QueryRecent(10, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Most recent first
	if events[0].EventType != EventFirewallWarn {
		t.Errorf("first event type = %q, want %q", events[0].EventType, EventFirewallWarn)
	}
}

func TestLogger_FilterByType(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "t1"})
	l.Log(EventFirewallBlock, "agent-2", FirewallDetails{Rule: "r1"})
	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "t2"})

	l.Close()

	events, err := l.QueryRecent(10, EventToolCall, "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 tool_call events, got %d", len(events))
	}
	for _, e := range events {
		if e.EventType != EventToolCall {
			t.Errorf("event type = %q, want %q", e.EventType, EventToolCall)
		}
	}
}

func TestLogger_FilterByAgent(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "t1"})
	l.Log(EventToolCall, "agent-2", ToolCallDetails{Tool: "t2"})
	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "t3"})

	l.Close()

	events, err := l.QueryRecent(10, "", "agent-1")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for agent-1, got %d", len(events))
	}
	for _, e := range events {
		if e.AgentName != "agent-1" {
			t.Errorf("agent = %q, want agent-1", e.AgentName)
		}
	}
}

func TestLogger_FilterByTypeAndAgent(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventToolCall, "agent-1", ToolCallDetails{Tool: "t1"})
	l.Log(EventFirewallWarn, "agent-1", FirewallDetails{Rule: "r1"})
	l.Log(EventToolCall, "agent-2", ToolCallDetails{Tool: "t2"})

	l.Close()

	events, err := l.QueryRecent(10, EventToolCall, "agent-1")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].AgentName != "agent-1" || events[0].EventType != EventToolCall {
		t.Errorf("unexpected event: agent=%q type=%q", events[0].AgentName, events[0].EventType)
	}
}

func TestLogger_Limit(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	for i := 0; i < 10; i++ {
		l.Log(EventToolCall, "agent", ToolCallDetails{Tool: "t"})
	}

	l.Close()

	events, err := l.QueryRecent(3, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events with limit, got %d", len(events))
	}
}

func TestLogger_DetailsUnmarshal(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventToolCall, "agent-1", ToolCallDetails{
		Tool:       "write_file",
		Server:     "filesystem",
		Status:     "ok",
		DurationMS: 150,
		Dangerous:  true,
		Args:       `{"path":"/tmp/test"}`,
	})

	l.Close()

	events, err := l.QueryRecent(1, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	var details ToolCallDetails
	if err := json.Unmarshal(events[0].Details, &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	if details.Tool != "write_file" {
		t.Errorf("tool = %q, want write_file", details.Tool)
	}
	if !details.Dangerous {
		t.Error("expected dangerous = true")
	}
	if details.DurationMS != 150 {
		t.Errorf("duration = %d, want 150", details.DurationMS)
	}
}

func TestLogger_ContentLog(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	l.Log(EventContentLog, "agent-1", ContentLogDetails{
		Direction: "request",
		Model:     "gpt-4o",
		Body:      `{"messages":[]}`,
	})

	l.Close()

	events, err := l.QueryRecent(1, EventContentLog, "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	var details ContentLogDetails
	if err := json.Unmarshal(events[0].Details, &details); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if details.Direction != "request" {
		t.Errorf("direction = %q, want request", details.Direction)
	}
}

func TestLogger_BatchFlush(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	const n = 75 // > maxBatch (50) to test batch boundary
	for i := 0; i < n; i++ {
		l.Log(EventToolCall, "agent", ToolCallDetails{Tool: "t"})
	}

	l.Close()

	events, err := l.QueryRecent(100, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != n {
		t.Errorf("expected %d events after batch flush, got %d", n, len(events))
	}
}

func TestLogger_TimestampPreserved(t *testing.T) {
	db := newTestDB(t)
	l := New(db, true, store.DialectSQLite)

	before := time.Now().UTC()
	l.Log(EventToolCall, "agent", ToolCallDetails{Tool: "t"})
	l.Close()
	after := time.Now().UTC()

	events, err := l.QueryRecent(1, "", "")
	if err != nil {
		t.Fatalf("QueryRecent() error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ts := events[0].Timestamp
	if ts.Before(before.Truncate(time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestSecureKeyMatch(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"equal", "secret-key-123", "secret-key-123", true},
		{"different", "secret-key-123", "secret-key-456", false},
		{"empty_both", "", "", true},
		{"empty_one", "key", "", false},
		{"different_length", "short", "longer-key", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SecureKeyMatch(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("SecureKeyMatch(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
