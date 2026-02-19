package audit

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// EventType constants for audit events.
const (
	EventToolCall      = "tool_call"
	EventFirewallBlock = "firewall_block"
	EventFirewallWarn  = "firewall_warn"
	EventContentLog    = "content_log"
)

// Event represents a single audit event.
type Event struct {
	ID        int64           `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	EventType string          `json:"event_type"`
	AgentName string          `json:"agent_name"`
	Details   json.RawMessage `json:"details"`
}

// ToolCallDetails holds details for a tool_call event.
type ToolCallDetails struct {
	Tool       string `json:"tool"`
	Server     string `json:"server"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Dangerous  bool   `json:"dangerous"`
	Args       string `json:"args,omitempty"`
}

// FirewallDetails holds details for firewall_block and firewall_warn events.
type FirewallDetails struct {
	Rule     string `json:"rule"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
}

// ContentLogDetails holds details for content_log events.
type ContentLogDetails struct {
	Direction string `json:"direction"`
	Model     string `json:"model"`
	Body      string `json:"body"`
}

// Logger writes audit events to the database asynchronously.
type Logger struct {
	db      *sql.DB
	enabled bool
	eventCh chan *Event
	done    chan struct{}
}

// New creates a new audit Logger. If not enabled, Log calls are no-ops.
func New(db *sql.DB, enabled bool) *Logger {
	l := &Logger{
		db:      db,
		enabled: enabled,
		eventCh: make(chan *Event, 256),
		done:    make(chan struct{}),
	}
	if enabled {
		go l.batchWriter()
	}
	return l
}

// Log records an audit event asynchronously. No-op if disabled.
func (l *Logger) Log(eventType, agentName string, details any) {
	if !l.enabled {
		return
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		log.Printf("ERROR: marshal audit details: %v", err)
		return
	}

	event := &Event{
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		AgentName: agentName,
		Details:   detailsJSON,
	}

	select {
	case l.eventCh <- event:
	default:
		// Channel full — insert synchronously as fallback
		if err := l.insert(event); err != nil {
			log.Printf("ERROR: audit fallback insert: %v", err)
		}
	}
}

// DB returns the underlying database connection.
func (l *Logger) DB() *sql.DB {
	return l.db
}

// QueryRecent returns recent audit events, optionally filtered.
func (l *Logger) QueryRecent(limit int, eventType, agentFilter string) ([]Event, error) {
	query := `SELECT id, timestamp, event_type, agent_name, details FROM audit_events`
	var conditions []string
	var args []any

	if eventType != "" {
		conditions = append(conditions, "event_type = ?")
		args = append(args, eventType)
	}
	if agentFilter != "" {
		conditions = append(conditions, "agent_name = ?")
		args = append(args, agentFilter)
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, c := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += c
		}
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var ts, details string
		if err := rows.Scan(&e.ID, &ts, &e.EventType, &e.AgentName, &details); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		e.Timestamp, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		e.Details = json.RawMessage(details)
		events = append(events, e)
	}
	return events, rows.Err()
}

// Close flushes pending events and stops the background writer.
func (l *Logger) Close() {
	if !l.enabled {
		return
	}
	close(l.eventCh)
	<-l.done
}

func (l *Logger) batchWriter() {
	defer close(l.done)

	const maxBatch = 50
	buf := make([]*Event, 0, maxBatch)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case e, ok := <-l.eventCh:
			if !ok {
				if len(buf) > 0 {
					l.insertBatch(buf)
				}
				return
			}
			buf = append(buf, e)
			if len(buf) >= maxBatch {
				l.insertBatch(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				l.insertBatch(buf)
				buf = buf[:0]
			}
		}
	}
}

func (l *Logger) insertBatch(events []*Event) {
	tx, err := l.db.Begin()
	if err != nil {
		log.Printf("ERROR: begin audit batch tx: %v", err)
		return
	}

	stmt, err := tx.Prepare(
		`INSERT INTO audit_events (timestamp, event_type, agent_name, details) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		log.Printf("ERROR: prepare audit batch stmt: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, e := range events {
		ts := e.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
		if _, err := stmt.Exec(ts, e.EventType, e.AgentName, string(e.Details)); err != nil {
			log.Printf("ERROR: audit batch insert: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: commit audit batch tx: %v", err)
	}
}

func (l *Logger) insert(e *Event) error {
	ts := e.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
	_, err := l.db.Exec(
		`INSERT INTO audit_events (timestamp, event_type, agent_name, details) VALUES (?, ?, ?, ?)`,
		ts, e.EventType, e.AgentName, string(e.Details),
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// SecureKeyMatch performs a constant-time string comparison to prevent timing attacks.
func SecureKeyMatch(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
