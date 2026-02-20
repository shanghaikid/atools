package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/agent-platform/agix/internal/store"
)

// Override holds per-session configuration overrides.
type Override struct {
	SessionID   string   `json:"session_id"`
	AgentName   string   `json:"agent_name"`
	Model       string   `json:"model,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Manager manages session-level config overrides.
type Manager struct {
	db         *sql.DB
	dialect    store.Dialect
	defaultTTL time.Duration
	done       chan struct{}
}

// New creates a Manager, initializes the table, and starts a background cleanup goroutine.
func New(db *sql.DB, defaultTTL time.Duration, dialect store.Dialect) (*Manager, error) {
	if err := createTable(db, dialect); err != nil {
		return nil, fmt.Errorf("create session_overrides table: %w", err)
	}
	m := &Manager{
		db:         db,
		dialect:    dialect,
		defaultTTL: defaultTTL,
		done:       make(chan struct{}),
	}
	go m.cleanupLoop()
	return m, nil
}

// Close stops the background cleanup goroutine.
func (m *Manager) Close() {
	close(m.done)
}

func createTable(db *sql.DB, dialect store.Dialect) error {
	if dialect == store.DialectPostgres {
		for _, stmt := range []string{
			`CREATE TABLE IF NOT EXISTS session_overrides (
				session_id  TEXT PRIMARY KEY,
				agent_name  TEXT NOT NULL DEFAULT '',
				model       TEXT,
				temperature DOUBLE PRECISION,
				max_tokens  INTEGER,
				created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
				expires_at  TIMESTAMP NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_session_expires ON session_overrides(expires_at)`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_overrides (
			session_id  TEXT PRIMARY KEY,
			agent_name  TEXT NOT NULL DEFAULT '',
			model       TEXT,
			temperature REAL,
			max_tokens  INTEGER,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			expires_at  DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_session_expires ON session_overrides(expires_at);
	`)
	return err
}

// ph returns the placeholder for position n (1-indexed).
func (m *Manager) ph(n int) string {
	if m.dialect == store.DialectPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// rebind rewrites ? placeholders for the dialect.
func (m *Manager) rebind(query string) string {
	return store.Rebind(m.dialect, query)
}

// nowExpr returns the SQL expression for "current time" for the dialect.
func (m *Manager) nowExpr() string {
	if m.dialect == store.DialectPostgres {
		return "NOW()"
	}
	return "datetime('now')"
}

// Get retrieves a non-expired session override.
func (m *Manager) Get(sessionID string) (*Override, error) {
	query := fmt.Sprintf(`
		SELECT session_id, agent_name, model, temperature, max_tokens, expires_at
		FROM session_overrides
		WHERE session_id = %s AND expires_at > %s
	`, m.ph(1), m.nowExpr())
	row := m.db.QueryRow(query, sessionID)

	var o Override
	var model sql.NullString
	var temp sql.NullFloat64
	var maxTok sql.NullInt64
	var expiresStr string

	err := row.Scan(&o.SessionID, &o.AgentName, &model, &temp, &maxTok, &expiresStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session override: %w", err)
	}

	if model.Valid {
		o.Model = model.String
	}
	if temp.Valid {
		v := temp.Float64
		o.Temperature = &v
	}
	if maxTok.Valid {
		v := int(maxTok.Int64)
		o.MaxTokens = &v
	}
	o.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresStr)

	return &o, nil
}

// Set upserts a session override. If ExpiresAt is zero, uses defaultTTL from now.
func (m *Manager) Set(o *Override) error {
	expiresAt := o.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(m.defaultTTL)
	}

	var temp *float64
	if o.Temperature != nil {
		temp = o.Temperature
	}
	var maxTok *int
	if o.MaxTokens != nil {
		maxTok = o.MaxTokens
	}
	var model *string
	if o.Model != "" {
		model = &o.Model
	}

	var query string
	if m.dialect == store.DialectPostgres {
		query = `INSERT INTO session_overrides (session_id, agent_name, model, temperature, max_tokens, expires_at)
			VALUES (?, ?, ?, ?, ?, ?::timestamp)
			ON CONFLICT (session_id) DO UPDATE SET agent_name = EXCLUDED.agent_name, model = EXCLUDED.model,
				temperature = EXCLUDED.temperature, max_tokens = EXCLUDED.max_tokens, expires_at = EXCLUDED.expires_at`
	} else {
		query = `INSERT OR REPLACE INTO session_overrides (session_id, agent_name, model, temperature, max_tokens, expires_at)
			VALUES (?, ?, ?, ?, ?, datetime(?))`
	}
	_, err := m.db.Exec(m.rebind(query), o.SessionID, o.AgentName, model, temp, maxTok, expiresAt.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("upsert session override: %w", err)
	}
	return nil
}

// Delete removes a session override.
func (m *Manager) Delete(sessionID string) error {
	_, err := m.db.Exec(m.rebind(`DELETE FROM session_overrides WHERE session_id = ?`), sessionID)
	if err != nil {
		return fmt.Errorf("delete session override: %w", err)
	}
	return nil
}

// CleanExpired removes all expired session overrides and returns the count deleted.
func (m *Manager) CleanExpired() (int64, error) {
	query := fmt.Sprintf(`DELETE FROM session_overrides WHERE expires_at <= %s`, m.nowExpr())
	result, err := m.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("clean expired sessions: %w", err)
	}
	return result.RowsAffected()
}

// ListActive returns all non-expired session overrides.
func (m *Manager) ListActive() ([]Override, error) {
	query := fmt.Sprintf(`
		SELECT session_id, agent_name, model, temperature, max_tokens, expires_at
		FROM session_overrides
		WHERE expires_at > %s
		ORDER BY expires_at ASC
	`, m.nowExpr())
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	var overrides []Override
	for rows.Next() {
		var o Override
		var model sql.NullString
		var temp sql.NullFloat64
		var maxTok sql.NullInt64
		var expiresStr string

		if err := rows.Scan(&o.SessionID, &o.AgentName, &model, &temp, &maxTok, &expiresStr); err != nil {
			return nil, fmt.Errorf("scan session override: %w", err)
		}
		if model.Valid {
			o.Model = model.String
		}
		if temp.Valid {
			v := temp.Float64
			o.Temperature = &v
		}
		if maxTok.Valid {
			v := int(maxTok.Int64)
			o.MaxTokens = &v
		}
		o.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresStr)
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

// Apply rewrites model, temperature, and max_tokens in the JSON request body.
func Apply(body []byte, o *Override) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	changed := false
	if o.Model != "" {
		modelJSON, _ := json.Marshal(o.Model)
		raw["model"] = modelJSON
		changed = true
	}
	if o.Temperature != nil {
		tempJSON, _ := json.Marshal(*o.Temperature)
		raw["temperature"] = tempJSON
		changed = true
	}
	if o.MaxTokens != nil {
		tokJSON, _ := json.Marshal(*o.MaxTokens)
		raw["max_tokens"] = tokJSON
		changed = true
	}

	if !changed {
		return body
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return out
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n, err := m.CleanExpired()
			if err != nil {
				log.Printf("WARN: session cleanup error: %v", err)
			} else if n > 0 {
				log.Printf("SESSION: cleaned %d expired override(s)", n)
			}
		case <-m.done:
			return
		}
	}
}
