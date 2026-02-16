package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
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
	TotalRequests  int
	TotalInput     int
	TotalOutput    int
	TotalCostUSD   float64
	AvgDurationMS  float64
	UniqueModels   int
	UniqueAgents   int
}

// AgentStats represents per-agent statistics.
type AgentStats struct {
	AgentName    string
	Requests     int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
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

// Store provides access to the SQLite database.
type Store struct {
	db       *sql.DB
	recordCh chan *Record
	done     chan struct{}
}

const createTableSQL = `
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
`

// New creates a new Store and initializes the schema.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	s := &Store{
		db:       db,
		recordCh: make(chan *Record, 256),
		done:     make(chan struct{}),
	}
	go s.batchWriter()
	return s, nil
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

// insertBatch inserts multiple records in a single transaction.
func (s *Store) insertBatch(records []*Record) {
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("ERROR: begin batch tx: %v", err)
		return
	}

	stmt, err := tx.Prepare(
		`INSERT INTO requests (timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code, failover_from, original_model)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
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
	// Store timestamp as ISO 8601 string so SQLite date functions work correctly
	ts := fmtTime(r.Timestamp)
	_, err := s.db.Exec(
		`INSERT INTO requests (timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code, failover_from, original_model)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ts, r.AgentName, r.Model, r.Provider, r.InputTokens, r.OutputTokens, r.CostUSD, r.DurationMS, r.StatusCode, r.FailoverFrom, r.OriginalModel,
	)
	if err != nil {
		return fmt.Errorf("insert record: %w", err)
	}
	return nil
}

// migrateSchema adds columns that may not exist in older databases.
func migrateSchema(db *sql.DB) error {
	// List of columns to add if missing: {name, type, default}
	migrations := []struct {
		column     string
		definition string
	}{
		{"failover_from", "TEXT NOT NULL DEFAULT ''"},
		{"original_model", "TEXT NOT NULL DEFAULT ''"},
	}

	for _, m := range migrations {
		if !columnExists(db, "requests", m.column) {
			stmt := fmt.Sprintf("ALTER TABLE requests ADD COLUMN %s %s", m.column, m.definition)
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("add column %s: %w", m.column, err)
			}
		}
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) bool {
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
		`SELECT
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COALESCE(AVG(duration_ms), 0),
			COUNT(DISTINCT model),
			COUNT(DISTINCT CASE WHEN agent_name != '' THEN agent_name END)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?`,
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
		`SELECT
			CASE WHEN agent_name = '' THEN '(unknown)' ELSE agent_name END,
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY agent_name
		 ORDER BY SUM(cost_usd) DESC`,
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
		`SELECT
			model,
			provider,
			COUNT(*),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY model
		 ORDER BY SUM(cost_usd) DESC`,
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

	rows, err := s.db.Query(query, args...)
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
	rows, err := s.db.Query(
		`SELECT
			date(timestamp) as day,
			COUNT(*),
			COALESCE(SUM(cost_usd), 0)
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 GROUP BY date(timestamp)
		 ORDER BY day`,
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
	Date     string
	Requests int
	CostUSD  float64
}

// QueryAgentDailySpend returns the total spend for an agent on a given day.
func (s *Store) QueryAgentDailySpend(agent string, day time.Time) (float64, error) {
	dateStr := day.Format("2006-01-02")
	row := s.db.QueryRow(
		`SELECT COALESCE(SUM(cost_usd), 0) FROM requests
		 WHERE agent_name = ? AND date(timestamp) = ?`,
		agent, dateStr,
	)
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
		`SELECT COALESCE(SUM(cost_usd), 0) FROM requests
		 WHERE agent_name = ? AND timestamp >= ? AND timestamp < ?`,
		agent, fmtTime(start), fmtTime(end),
	)
	var cost float64
	if err := row.Scan(&cost); err != nil {
		return 0, fmt.Errorf("query agent monthly spend: %w", err)
	}
	return cost, nil
}

// ExportCSV returns all records in the time range for CSV export.
func (s *Store) ExportCSV(since, until time.Time) ([]Record, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, agent_name, model, provider, input_tokens, output_tokens, cost_usd, duration_ms, status_code
		 FROM requests
		 WHERE timestamp >= ? AND timestamp <= ?
		 ORDER BY timestamp ASC`,
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
