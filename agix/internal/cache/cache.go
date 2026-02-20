package cache

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/agent-platform/agix/internal/store"
)

// Config defines cache settings.
type Config struct {
	Enabled             bool    `yaml:"enabled"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	TTLMinutes          int     `yaml:"ttl_minutes"`
}

// Entry represents a cached response.
type Entry struct {
	Hash       string
	Model      string
	Response   []byte
	Embedding  []float32
	CreatedAt  time.Time
}

// LookupResult contains cache lookup results.
type LookupResult struct {
	Hit      bool
	Response []byte
	Method   string // "exact" or "semantic"
}

// Cache provides exact and semantic response caching.
type Cache struct {
	db        *sql.DB
	dialect   store.Dialect
	embedder  *EmbeddingClient
	threshold float64
	ttl       time.Duration
}

const createCacheTableSQLite = `
CREATE TABLE IF NOT EXISTS cache_entries (
	hash       TEXT NOT NULL,
	model      TEXT NOT NULL,
	response   BLOB NOT NULL,
	embedding  BLOB,
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	PRIMARY KEY (hash, model)
);

CREATE INDEX IF NOT EXISTS idx_cache_model ON cache_entries(model);
CREATE INDEX IF NOT EXISTS idx_cache_created ON cache_entries(created_at);
`

var createCacheTablePostgres = []string{
	`CREATE TABLE IF NOT EXISTS cache_entries (
		hash       TEXT NOT NULL,
		model      TEXT NOT NULL,
		response   BYTEA NOT NULL,
		embedding  BYTEA,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		PRIMARY KEY (hash, model)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cache_model ON cache_entries(model)`,
	`CREATE INDEX IF NOT EXISTS idx_cache_created ON cache_entries(created_at)`,
}

// New creates a new Cache. Returns nil if not enabled.
func New(cfg Config, db *sql.DB, embedder *EmbeddingClient, dialect store.Dialect) (*Cache, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if cfg.SimilarityThreshold <= 0 {
		cfg.SimilarityThreshold = 0.95
	}
	if cfg.TTLMinutes <= 0 {
		cfg.TTLMinutes = 60
	}

	if dialect == store.DialectPostgres {
		for _, stmt := range createCacheTablePostgres {
			if _, err := db.Exec(stmt); err != nil {
				return nil, fmt.Errorf("create cache table: %w", err)
			}
		}
	} else {
		if _, err := db.Exec(createCacheTableSQLite); err != nil {
			return nil, fmt.Errorf("create cache table: %w", err)
		}
	}

	return &Cache{
		db:        db,
		dialect:   dialect,
		embedder:  embedder,
		threshold: cfg.SimilarityThreshold,
		ttl:       time.Duration(cfg.TTLMinutes) * time.Minute,
	}, nil
}

// Lookup checks the cache for a matching response.
// It first tries an exact SHA-256 match, then falls back to semantic similarity.
func (c *Cache) Lookup(model string, messages json.RawMessage) LookupResult {
	contentKey := extractContentKey(messages)
	hash := sha256Hash(contentKey)

	// Exact match
	entry, err := c.getExact(hash, model)
	if err == nil && entry != nil {
		if time.Since(entry.CreatedAt) < c.ttl {
			return LookupResult{Hit: true, Response: entry.Response, Method: "exact"}
		}
		// Expired — delete
		c.deleteEntry(hash, model)
	}

	// Semantic match (requires embedder)
	if c.embedder == nil {
		return LookupResult{Hit: false}
	}

	queryEmbedding, err := c.embedder.Embed(contentKey)
	if err != nil {
		log.Printf("CACHE: embedding error: %v", err)
		return LookupResult{Hit: false}
	}

	bestEntry, bestSim := c.findSemantic(model, queryEmbedding)
	if bestEntry != nil && bestSim >= c.threshold {
		if time.Since(bestEntry.CreatedAt) < c.ttl {
			log.Printf("CACHE: semantic hit (similarity: %.4f)", bestSim)
			return LookupResult{Hit: true, Response: bestEntry.Response, Method: "semantic"}
		}
	}

	return LookupResult{Hit: false}
}

// Store saves a response in the cache.
func (c *Cache) Store(model string, messages json.RawMessage, response []byte) {
	contentKey := extractContentKey(messages)
	hash := sha256Hash(contentKey)

	var embeddingBlob []byte
	if c.embedder != nil {
		embedding, err := c.embedder.Embed(contentKey)
		if err != nil {
			log.Printf("CACHE: embedding error on store: %v", err)
		} else {
			embeddingBlob = encodeEmbedding(embedding)
		}
	}

	var query string
	if c.dialect == store.DialectPostgres {
		query = `INSERT INTO cache_entries (hash, model, response, embedding, created_at) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (hash, model) DO UPDATE SET response = EXCLUDED.response, embedding = EXCLUDED.embedding, created_at = EXCLUDED.created_at`
	} else {
		query = `INSERT OR REPLACE INTO cache_entries (hash, model, response, embedding, created_at) VALUES (?, ?, ?, ?, ?)`
	}
	_, err := c.db.Exec(
		store.Rebind(c.dialect, query),
		hash, model, response, embeddingBlob, time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		log.Printf("CACHE: store error: %v", err)
	}
}

// Cleanup removes expired cache entries.
func (c *Cache) Cleanup() {
	cutoff := time.Now().UTC().Add(-c.ttl).Format("2006-01-02T15:04:05Z")
	_, err := c.db.Exec(store.Rebind(c.dialect, `DELETE FROM cache_entries WHERE created_at < ?`), cutoff)
	if err != nil {
		log.Printf("CACHE: cleanup error: %v", err)
	}
}

func (c *Cache) getExact(hash, model string) (*Entry, error) {
	row := c.db.QueryRow(
		store.Rebind(c.dialect, `SELECT hash, model, response, embedding, created_at FROM cache_entries WHERE hash = ? AND model = ?`),
		hash, model,
	)
	var e Entry
	var embBlob []byte
	var ts string
	if err := row.Scan(&e.Hash, &e.Model, &e.Response, &embBlob, &ts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	e.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", ts)
	if embBlob != nil {
		e.Embedding = decodeEmbedding(embBlob)
	}
	return &e, nil
}

func (c *Cache) findSemantic(model string, queryEmb []float32) (*Entry, float64) {
	rows, err := c.db.Query(
		store.Rebind(c.dialect, `SELECT hash, model, response, embedding, created_at FROM cache_entries WHERE model = ? AND embedding IS NOT NULL`),
		model,
	)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	var bestEntry *Entry
	bestSim := -1.0

	for rows.Next() {
		var e Entry
		var embBlob []byte
		var ts string
		if err := rows.Scan(&e.Hash, &e.Model, &e.Response, &embBlob, &ts); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		if embBlob == nil {
			continue
		}
		e.Embedding = decodeEmbedding(embBlob)

		sim := cosineSimilarity(queryEmb, e.Embedding)
		if sim > bestSim {
			bestSim = sim
			entryCopy := e
			bestEntry = &entryCopy
		}
	}

	return bestEntry, bestSim
}

func (c *Cache) deleteEntry(hash, model string) {
	c.db.Exec(store.Rebind(c.dialect, `DELETE FROM cache_entries WHERE hash = ? AND model = ?`), hash, model)
}

// extractContentKey builds a cache key from user message content.
func extractContentKey(messages json.RawMessage) string {
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(messages, &msgs); err != nil {
		return string(messages)
	}

	var parts []string
	for _, m := range msgs {
		if m.Role == "user" {
			parts = append(parts, m.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// encodeEmbedding converts a float32 slice to a binary blob (little-endian).
func encodeEmbedding(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// decodeEmbedding converts a binary blob to a float32 slice.
func decodeEmbedding(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
