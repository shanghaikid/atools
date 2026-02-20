package cache

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-platform/agix/internal/store"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNew_NilWhenDisabled(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: false}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Error("expected nil when disabled")
	}
}

func TestNew_Defaults(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: true}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.threshold != 0.95 {
		t.Errorf("threshold = %f, want 0.95", c.threshold)
	}
	if c.ttl != 60*time.Minute {
		t.Errorf("ttl = %v, want 60m", c.ttl)
	}
}

func TestExactMatch(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: true, TTLMinutes: 60}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "What is 2+2?"},
	})
	response := []byte(`{"choices":[{"message":{"content":"4"}}]}`)

	// Store
	c.Store("gpt-4o", msgs, response)

	// Lookup — should hit
	result := c.Lookup("gpt-4o", msgs)
	if !result.Hit {
		t.Fatal("expected cache hit")
	}
	if result.Method != "exact" {
		t.Errorf("Method = %q, want %q", result.Method, "exact")
	}
	if string(result.Response) != string(response) {
		t.Error("response mismatch")
	}
}

func TestExactMatch_DifferentModel(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: true, TTLMinutes: 60}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "What is 2+2?"},
	})
	response := []byte(`{"choices":[{"message":{"content":"4"}}]}`)

	c.Store("gpt-4o", msgs, response)

	// Different model — should miss
	result := c.Lookup("claude-sonnet-4-20250514", msgs)
	if result.Hit {
		t.Error("expected miss for different model")
	}
}

func TestExactMatch_Expired(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: true, TTLMinutes: 1}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	msgs, _ := json.Marshal([]map[string]string{
		{"role": "user", "content": "What is 2+2?"},
	})
	response := []byte(`{"choices":[{"message":{"content":"4"}}]}`)

	// Insert with past timestamp
	contentKey := extractContentKey(msgs)
	hash := sha256Hash(contentKey)
	past := time.Now().UTC().Add(-2 * time.Minute).Format("2006-01-02T15:04:05Z")
	_, err = db.Exec(
		`INSERT INTO cache_entries (hash, model, response, created_at) VALUES (?, ?, ?, ?)`,
		hash, "gpt-4o", response, past,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	result := c.Lookup("gpt-4o", msgs)
	if result.Hit {
		t.Error("expected miss for expired entry")
	}
}

func TestCleanup(t *testing.T) {
	db := openTestDB(t)
	c, err := New(Config{Enabled: true, TTLMinutes: 1}, db, nil, store.DialectSQLite)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Insert expired entry
	past := time.Now().UTC().Add(-2 * time.Minute).Format("2006-01-02T15:04:05Z")
	_, err = db.Exec(
		`INSERT INTO cache_entries (hash, model, response, created_at) VALUES (?, ?, ?, ?)`,
		"abc", "gpt-4o", []byte("resp"), past,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	c.Cleanup()

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM cache_entries`).Scan(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0 after cleanup", count)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim < 0.999 {
		t.Errorf("identical vectors: sim = %f, want ~1.0", sim)
	}

	c := []float32{0, 1, 0}
	sim = cosineSimilarity(a, c)
	if sim > 0.001 {
		t.Errorf("orthogonal vectors: sim = %f, want ~0.0", sim)
	}
}

func TestEncodeDecodeEmbedding(t *testing.T) {
	original := []float32{1.0, 2.5, -3.14, 0.0}
	blob := encodeEmbedding(original)
	decoded := decodeEmbedding(blob)

	if len(decoded) != len(original) {
		t.Fatalf("len = %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("[%d] = %f, want %f", i, decoded[i], original[i])
		}
	}
}

func TestExtractContentKey(t *testing.T) {
	msgs, _ := json.Marshal([]map[string]string{
		{"role": "system", "content": "You are helpful."},
		{"role": "user", "content": "Hello"},
		{"role": "assistant", "content": "Hi!"},
		{"role": "user", "content": "How are you?"},
	})
	key := extractContentKey(msgs)
	if key != "Hello\nHow are you?" {
		t.Errorf("key = %q, want %q", key, "Hello\nHow are you?")
	}
}
