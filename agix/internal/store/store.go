package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Record represents a single API call record.
type Record struct {
	ID           int64
	Timestamp    time.Time
	AgentName    string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMS   int64
	StatusCode    int
	FailoverFrom  string
	OriginalModel string
}

// Stats represents aggregated statistics.
type Stats struct {
	TotalRequests  int     `json:"total_requests"`
	TotalInput     int     `json:"total_input"`
	TotalOutput    int     `json:"total_output"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	AvgDurationMS  float64 `json:"avg_duration_ms"`
	UniqueModels   int     `json:"unique_models"`
	UniqueAgents   int     `json:"unique_agents"`
}

// AgentStats represents per-agent statistics.
type AgentStats struct {
	AgentName    string  `json:"agent_name"`
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// ModelStats represents per-model statistics.
type ModelStats struct {
	Model        string
	Provider     string
	Requests     int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// Store provides access to the database (SQLite or PostgreSQL).
type Store struct {
	db       *sql.DB
	dialect  Dialect
	recordCh chan *Record
	done     chan struct{}
}

const createTableSQLite = `
CREATE TABLE IF NOT EXISTS requests (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp    DATETIME NOT NULL DEFAULT (datetime('now')),
	agent_name   TEXT NOT NULL DEFAULT '',
	model        TEXT NOT NULL,
	provider     TEXT NOT NULL,
	input_tokens  INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd     REAL NOT NULL DEFAULT 0,
	duration_ms  INTEGER NOT NULL DEFAULT 0,
	status_code  INTEGER NOT NULL DEFAULT 200
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_requests_agent ON requests(agent_name);
CREATE INDEX IF NOT EXISTS idx_requests_model ON requests(model);

CREATE TABLE IF NOT EXISTS traces (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	trace_id   TEXT NOT NULL UNIQUE,
	agent_name TEXT NOT NULL DEFAULT '',
	model      TEXT NOT NULL DEFAULT '',
	timestamp  DATETIME NOT NULL,
	spans      TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_traces_trace_id ON traces(trace_id);
CREATE INDEX IF NOT EXISTS idx_traces_timestamp ON traces(timestamp);

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

CREATE TABLE IF NOT EXISTS webhook_executions (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp     DATETIME NOT NULL DEFAULT (datetime('now')),
	webhook_name  TEXT NOT NULL,
	status        TEXT NOT NULL DEFAULT 'pending',
	payload       TEXT NOT NULL DEFAULT '',
	result        TEXT NOT NULL DEFAULT '',
	error         TEXT NOT NULL DEFAULT '',
	duration_ms   INTEGER NOT NULL DEFAULT 0,
	callback_code INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_webhook_executions_name ON webhook_executions(webhook_name);
CREATE INDEX IF NOT EXISTS idx_webhook_executions_timestamp ON webhook_executions(timestamp);
`

// postgresCreateStatements are executed one at a time (PostgreSQL cannot run
// multiple DDL statements in a single Exec call as reliably as SQLite).
var postgresCreateStatements = []string{
	`CREATE TABLE IF NOT EXISTS requests (
		id            BIGSERIAL PRIMARY KEY,
		timestamp     TIMESTAMP NOT NULL DEFAULT NOW(),
		agent_name    TEXT NOT NULL DEFAULT '',
		model         TEXT NOT NULL,
		provider      TEXT NOT NULL,
		input_tokens  INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd      DOUBLE PRECISION NOT NULL DEFAULT 0,
		duration_ms   BIGINT NOT NULL DEFAULT 0,
		status_code   INTEGER NOT NULL DEFAULT 200,
		failover_from  TEXT NOT NULL DEFAULT '',
		original_model TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_requests_agent ON requests(agent_name)`,
	`CREATE INDEX IF NOT EXISTS idx_requests_model ON requests(model)`,
	`CREATE TABLE IF NOT EXISTS traces (
		id         BIGSERIAL PRIMARY KEY,
		trace_id   TEXT NOT NULL UNIQUE,
		agent_name TEXT NOT NULL DEFAULT '',
		model      TEXT NOT NULL DEFAULT '',
		timestamp  TIMESTAMP NOT NULL,
		spans      TEXT NOT NULL DEFAULT '[]'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_traces_trace_id ON traces(trace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_traces_timestamp ON traces(timestamp)`,
	`CREATE TABLE IF NOT EXISTS audit_events (
		id          BIGSERIAL PRIMARY KEY,
		timestamp   TIMESTAMP NOT NULL,
		event_type  TEXT NOT NULL,
		agent_name  TEXT NOT NULL DEFAULT '',
		details     TEXT NOT NULL DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_type ON audit_events(event_type)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_events_agent ON audit_events(agent_name)`,
	`CREATE TABLE IF NOT EXISTS webhook_executions (
		id            BIGSERIAL PRIMARY KEY,
		timestamp     TIMESTAMP NOT NULL DEFAULT NOW(),
		webhook_name  TEXT NOT NULL,
		status        TEXT NOT NULL DEFAULT 'pending',
		payload       TEXT NOT NULL DEFAULT '',
		result        TEXT NOT NULL DEFAULT '',
		error         TEXT NOT NULL DEFAULT '',
		duration_ms   BIGINT NOT NULL DEFAULT 0,
		callback_code INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_webhook_executions_name ON webhook_executions(webhook_name)`,
	`CREATE INDEX IF NOT EXISTS idx_webhook_executions_timestamp ON webhook_executions(timestamp)`,
}

// New creates a new Store and initializes the schema.
func New(dsn string) (*Store, error) {
	db, dialect, err := OpenDB(dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := createSchema(db, dialect); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	if err := migrateSchema(db, dialect); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	s := &Store{
		db:       db,
		dialect:  dialect,
		recordCh: make(chan *Record, 256),
		done:     make(chan struct{}),
	}
	go s.batchWriter()
	return s, nil
}

func createSchema(db *sql.DB, dialect Dialect) error {
	if dialect == DialectPostgres {
		for _, stmt := range postgresCreateStatements {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("exec DDL: %w", err)
			}
		}
		return nil
	}
	_, err := db.Exec(createTableSQLite)
	return err
}

// Dialect returns the dialect used by this store.
func (s *Store) Dialect() Dialect {
	return s.dialect
}

// DB returns the underlying *sql.DB for use by other packages (e.g., cache).
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close flushes pending async writes and closes the database connection.
func (s *Store) Close() error {
	close(s.recordCh)
	<-s.done
	return s.db.Close()
}

// InsertAsync queues a record for asynchronous batch insertion.
// If the channel is full, it falls back to a synchronous insert.
func (s *Store) InsertAsync(r *Record) {
	select {
	case s.recordCh <- r:
	default:
		if err := s.Insert(r); err != nil {
			log.Printf("ERROR: async fallback insert failed: %v", err)
		}
	}
}

// batchWriter drains the record channel, flushing in batches of up to 50
// or after 1 second of inactivity.
func (s *Store) batchWriter() {
	defer close(s.done)

	const maxBatch = 50
	buf := make([]*Record, 0, maxBatch)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case r, ok := <-s.recordCh:
			if !ok {
				// Channel closed — flush remaining records.
				if len(buf) > 0 {
					s.insertBatch(buf)
				}
				return
			}
			buf = append(buf, r)
			if len(buf) >= maxBatch {
				s.insertBatch(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				s.insertBatch(buf)
				buf = buf[:0]
			}
		}
	}
}

const insertRequestSQL = `INSERT INTO requests (timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code, failover_from, original_model)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// insertBatch inserts multiple records in a single transaction.
func (s *Store) insertBatch(records []*Record) {
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("ERROR: begin batch tx: %v", err)
		return
	}

	stmt, err := tx.Prepare(Rebind(s.dialect, insertRequestSQL))
	if err != nil {
		log.Printf("ERROR: prepare batch stmt: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, r := range records {
		ts := fmtTime(r.Timestamp)
		if _, err := stmt.Exec(ts, r.AgentName, r.Model, r.Provider, r.InputTokens, r.OutputTokens, r.CostUSD, r.DurationMS, r.StatusCode, r.FailoverFrom, r.OriginalModel); err != nil {
			log.Printf("ERROR: batch insert record: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: commit batch tx: %v", err)
	}
}

// Insert records a new API call.
func (s *Store) Insert(r *Record) error {
	ts := fmtTime(r.Timestamp)
	_, err := s.db.Exec(
		Rebind(s.dialect, insertRequestSQL),
		ts, r.AgentName, r.Model, r.Provider, r.InputTokens, r.OutputTokens, r.CostUSD, r.DurationMS, r.StatusCode, r.FailoverFrom, r.OriginalModel,
	)
	if err != nil {
		return fmt.Errorf("insert record: %w", err)
	}
	return nil
}

// migrateSchema adds columns that may not exist in older databases.
func migrateSchema(db *sql.DB, dialect Dialect) error {
	// PostgreSQL DDL already includes these columns, so migration is only needed for SQLite.
	if dialect == DialectPostgres {
		return nil
	}

	migrations := []struct {
		column     string
		definition string
	}{
		{"failover_from", "TEXT NOT NULL DEFAULT ''"},
		{"original_model", "TEXT NOT NULL DEFAULT ''"},
	}

	for _, m := range migrations {
		if !columnExists(db, "requests", m.column, dialect) {
			stmt := fmt.Sprintf("ALTER TABLE requests ADD COLUMN %s %s", m.column, m.definition)
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("add column %s: %w", m.column, err)
			}
		}
	}
	return nil
}

func columnExists(db *sql.DB, table, column string, dialect Dialect) bool {
	if dialect == DialectPostgres {
		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
			table, column,
		).Scan(&exists)
		return err == nil && exists
	}

	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

const timeFormat = "2006-01-02T15:04:05Z"

func fmtTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

// QueryStats returns aggregated stats, optionally filtered by time range.
func (s *Store) QueryStats(since, until time.Time) (*Stats, error) {
	row := s.db.QueryRow(
		Rebind(s.dialect, `SELECT
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(AVG(duration_ms), 0),
			COUNT(DISTINCT model),
			COUNT(DISTINCT CASE WHEN agent_name != '' THEN agent_name END)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?`),
		fmtTime(since), fmtTime(until),
	)

	var st Stats
	err := row.Scan(&st.TotalRequests, &st.TotalInput, &st.TotalOutput, &st.TotalCostUSD, &st.AvgDurationMS, &st.UniqueModels, &st.UniqueAgents)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	return &st, nil
}

// QueryStatsByAgent returns stats grouped by agent.
func (s *Store) QueryStatsByAgent(since, until time.Time) ([]AgentStats, error) {
	rows, err := s.db.Query(
		Rebind(s.dialect, `SELECT
			CASE WHEN agent_name = '' THEN '(unknown)' ELSE agent_name END,
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY agent_name
		 ORDER BY SUM(cost_usd) DESC`),
		fmtTime(since), fmtTime(until),
	)
	if err != nil {
		return nil, fmt.Errorf("query agent stats: %w", err)
	}
	defer rows.Close()

	var results []AgentStats
	for rows.Next() {
		var a AgentStats
		if err := rows.Scan(&a.AgentName, &a.Requests, &a.InputTokens, &a.OutputTokens, &a.CostUSD); err != nil {
			return nil, fmt.Errorf("scan agent stats: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// QueryStatsByModel returns stats grouped by model.
func (s *Store) QueryStatsByModel(since, until time.Time) ([]ModelStats, error) {
	rows, err := s.db.Query(
		Rebind(s.dialect, `SELECT
			model,
			provider,
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY model
		 ORDER BY SUM(cost_usd) DESC`),
		fmtTime(since), fmtTime(until),
	)
	if err != nil {
		return nil, fmt.Errorf("query model stats: %w", err)
	}
	defer rows.Close()

	var results []ModelStats
	for rows.Next() {
		var m ModelStats
		if err := rows.Scan(&m.Model, &m.Provider, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.CostUSD); err != nil {
			return nil, fmt.Errorf("scan model stats: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// QueryRecentRequests returns the most recent N requests.
func (s *Store) QueryRecentRequests(limit int, agentFilter string) ([]Record, error) {
	query := `SELECT id, timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code
		 FROM requests`
	args := []any{}

	if agentFilter != "" {
		query += ` WHERE agent_name = ?`
		args = append(args, agentFilter)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(Rebind(s.dialect, query), args...)
	if err != nil {
		return nil, fmt.Errorf("query recent requests: %w", err)
	}
	defer rows.Close()

	var results []Record
	for rows.Next() {
		var r Record
		var ts string
		if err := rows.Scan(&r.ID, &ts, &r.AgentName, &r.Model, &r.Provider, &r.InputTokens, &r.OutputTokens, &r.CostUSD, &r.DurationMS, &r.StatusCode); err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		r.Timestamp, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		results = append(results, r)
	}
	return results, rows.Err()
}

// QueryDailyCosts returns daily cost totals for the given period.
func (s *Store) QueryDailyCosts(since, until time.Time) ([]DailyCost, error) {
	dateExpr := "date(timestamp)"
	if s.dialect == DialectPostgres {
		dateExpr = "timestamp::date"
	}
	query := fmt.Sprintf(`SELECT
			%s as day,
			COUNT(*),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY %s
		 ORDER BY day`, dateExpr, dateExpr)
	rows, err := s.db.Query(
		Rebind(s.dialect, query),
		fmtTime(since), fmtTime(until),
	)
	if err != nil {
		return nil, fmt.Errorf("query daily costs: %w", err)
	}
	defer rows.Close()

	var results []DailyCost
	for rows.Next() {
		var d DailyCost
		if err := rows.Scan(&d.Date, &d.Requests, &d.CostUSD); err != nil {
			return nil, fmt.Errorf("scan daily cost: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// DailyCost represents aggregated costs for a single day.
type DailyCost struct {
	Date     string  `json:"date"`
	Requests int     `json:"requests"`
	CostUSD  float64 `json:"cost_usd"`
}

// QueryAgentDailySpend returns the total spend for an agent on a given day.
func (s *Store) QueryAgentDailySpend(agent string, day time.Time) (float64, error) {
	dateStr := day.Format("2006-01-02")
	dateExpr := "date(timestamp)"
	if s.dialect == DialectPostgres {
		dateExpr = "timestamp::date"
	}
	query := fmt.Sprintf(`SELECT COALESCE(SUM(cost_usd), 0) FROM requests
		 WHERE agent_name = ? AND %s = ?`, dateExpr)
	row := s.db.QueryRow(Rebind(s.dialect, query), agent, dateStr)
	var cost float64
	if err := row.Scan(&cost); err != nil {
		return 0, fmt.Errorf("query agent daily spend: %w", err)
	}
	return cost, nil
}

// QueryAgentMonthlySpend returns the total spend for an agent in a given month.
func (s *Store) QueryAgentMonthlySpend(agent string, year int, month time.Month) (float64, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	row := s.db.QueryRow(
		Rebind(s.dialect, `SELECT COALESCE(SUM(cost_usd), 0) FROM requests
		 WHERE agent_name = ? AND timestamp >= ? AND timestamp < ?`),
		agent, fmtTime(start), fmtTime(end),
	)
	var cost float64
	if err := row.Scan(&cost); err != nil {
		return 0, fmt.Errorf("query agent monthly spend: %w", err)
	}
	return cost, nil
}

// TraceRecord represents a stored request trace.
type TraceRecord struct {
	TraceID   string `json:"trace_id"`
	AgentName string `json:"agent_name"`
	Model     string `json:"model"`
	Timestamp time.Time `json:"timestamp"`
	Spans     json.RawMessage `json:"spans"`
}

// InsertTrace stores a trace record.
func (s *Store) InsertTrace(traceID, agentName, model string, timestamp time.Time, spansJSON []byte) error {
	_, err := s.db.Exec(
		Rebind(s.dialect, `INSERT INTO traces (trace_id, agent_name, model, timestamp, spans) VALUES (?, ?, ?, ?, ?)`),
		traceID, agentName, model, fmtTime(timestamp), string(spansJSON),
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}
	return nil
}

// QueryTrace returns a single trace by its trace ID.
func (s *Store) QueryTrace(traceID string) (*TraceRecord, error) {
	row := s.db.QueryRow(
		Rebind(s.dialect, `SELECT trace_id, agent_name, model, timestamp, spans FROM traces WHERE trace_id = ?`),
		traceID,
	)
	var tr TraceRecord
	var ts, spans string
	if err := row.Scan(&tr.TraceID, &tr.AgentName, &tr.Model, &ts, &spans); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query trace: %w", err)
	}
	tr.Timestamp, _ = time.Parse(timeFormat, ts)
	tr.Spans = json.RawMessage(spans)
	return &tr, nil
}

// QueryRecentTraces returns the most recent N traces, optionally filtered by agent.
func (s *Store) QueryRecentTraces(limit int, agentFilter string) ([]TraceRecord, error) {
	query := `SELECT trace_id, agent_name, model, timestamp, spans FROM traces`
	args := []any{}

	if agentFilter != "" {
		query += ` WHERE agent_name = ?`
		args = append(args, agentFilter)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(Rebind(s.dialect, query), args...)
	if err != nil {
		return nil, fmt.Errorf("query recent traces: %w", err)
	}
	defer rows.Close()

	var results []TraceRecord
	for rows.Next() {
		var tr TraceRecord
		var ts, spans string
		if err := rows.Scan(&tr.TraceID, &tr.AgentName, &tr.Model, &ts, &spans); err != nil {
			return nil, fmt.Errorf("scan trace: %w", err)
		}
		tr.Timestamp, _ = time.Parse(timeFormat, ts)
		tr.Spans = json.RawMessage(spans)
		results = append(results, tr)
	}
	return results, rows.Err()
}

// WebhookExecution represents a webhook execution record.
type WebhookExecution struct {
	ID           int64  `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	WebhookName  string `json:"webhook_name"`
	Status       string `json:"status"`
	Payload      string `json:"payload"`
	Result       string `json:"result"`
	Error        string `json:"error"`
	DurationMS   int64  `json:"duration_ms"`
	CallbackCode int    `json:"callback_code"`
}

// InsertWebhookExecution inserts a new webhook execution record and returns its ID.
func (s *Store) InsertWebhookExecution(name, status, payload string) (int64, error) {
	if s.dialect == DialectPostgres {
		var id int64
		err := s.db.QueryRow(
			`INSERT INTO webhook_executions (timestamp, webhook_name, status, payload) VALUES ($1, $2, $3, $4) RETURNING id`,
			fmtTime(time.Now().UTC()), name, status, payload,
		).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("insert webhook execution: %w", err)
		}
		return id, nil
	}
	result, err := s.db.Exec(
		`INSERT INTO webhook_executions (timestamp, webhook_name, status, payload) VALUES (?, ?, ?, ?)`,
		fmtTime(time.Now().UTC()), name, status, payload,
	)
	if err != nil {
		return 0, fmt.Errorf("insert webhook execution: %w", err)
	}
	return result.LastInsertId()
}

// UpdateWebhookExecution updates an existing webhook execution record.
func (s *Store) UpdateWebhookExecution(id int64, status, resultText, errText string, durationMS int64, callbackCode int) error {
	_, err := s.db.Exec(
		Rebind(s.dialect, `UPDATE webhook_executions SET status = ?, result = ?, error = ?, duration_ms = ?, callback_code = ? WHERE id = ?`),
		status, resultText, errText, durationMS, callbackCode, id,
	)
	if err != nil {
		return fmt.Errorf("update webhook execution: %w", err)
	}
	return nil
}

// QueryWebhookExecutions returns recent webhook executions, optionally filtered by name.
func (s *Store) QueryWebhookExecutions(limit int, nameFilter string) ([]WebhookExecution, error) {
	query := `SELECT id, timestamp, webhook_name, status, payload, result, error, duration_ms, callback_code FROM webhook_executions`
	args := []any{}

	if nameFilter != "" {
		query += ` WHERE webhook_name = ?`
		args = append(args, nameFilter)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(Rebind(s.dialect, query), args...)
	if err != nil {
		return nil, fmt.Errorf("query webhook executions: %w", err)
	}
	defer rows.Close()

	var results []WebhookExecution
	for rows.Next() {
		var we WebhookExecution
		var ts string
		if err := rows.Scan(&we.ID, &ts, &we.WebhookName, &we.Status, &we.Payload, &we.Result, &we.Error, &we.DurationMS, &we.CallbackCode); err != nil {
			return nil, fmt.Errorf("scan webhook execution: %w", err)
		}
		we.Timestamp, _ = time.Parse(timeFormat, ts)
		results = append(results, we)
	}
	return results, rows.Err()
}

// ExportCSV returns all records in the time range for CSV export.
func (s *Store) ExportCSV(since, until time.Time) ([]Record, error) {
	rows, err := s.db.Query(
		Rebind(s.dialect, `SELECT id, timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`),
		fmtTime(since), fmtTime(until),
	)
	if err != nil {
		return nil, fmt.Errorf("export records: %w", err)
	}
	defer rows.Close()

	var results []Record
	for rows.Next() {
		var r Record
		var ts string
		if err := rows.Scan(&r.ID, &ts, &r.AgentName, &r.Model, &r.Provider, &r.InputTokens, &r.OutputTokens, &r.CostUSD, &r.DurationMS, &r.StatusCode); err != nil {
			return nil, fmt.Errorf("scan export record: %w", err)
		}
		r.Timestamp, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		results = append(results, r)
	}
	return results, rows.Err()
}
