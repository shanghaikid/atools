package trace

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Trace collects spans for a single request pipeline.
type Trace struct {
	ID        string    `json:"trace_id"`
	Timestamp time.Time `json:"timestamp"`
	AgentName string    `json:"agent_name"`
	Model     string    `json:"model"`

	mu    sync.Mutex
	spans []Span
}

// Span records a single pipeline step.
type Span struct {
	Name       string         `json:"name"`
	StartTime  time.Time      `json:"start_time"`
	DurationMS int64          `json:"duration_ms"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// SpanHandle is a builder for recording a span.
type SpanHandle struct {
	trace    *Trace
	name     string
	start    time.Time
	metadata map[string]any
}

// New creates a new Trace with a random 12-char hex ID.
func New() *Trace {
	b := make([]byte, 6)
	rand.Read(b)
	return &Trace{
		ID:        hex.EncodeToString(b),
		Timestamp: time.Now().UTC(),
	}
}

// Spans returns a copy of the recorded spans.
func (t *Trace) Spans() []Span {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Span, len(t.spans))
	copy(out, t.spans)
	return out
}

// StartSpan begins a new span. Call End() on the returned handle to finish it.
// Nil-safe: returns a no-op handle if t is nil.
func (t *Trace) StartSpan(name string) *SpanHandle {
	if t == nil {
		return &SpanHandle{}
	}
	return &SpanHandle{
		trace:    t,
		name:     name,
		start:    time.Now(),
		metadata: map[string]any{},
	}
}

// Set adds a key-value pair to the span metadata. Returns the handle for chaining.
func (h *SpanHandle) Set(key string, value any) *SpanHandle {
	if h.trace == nil {
		return h
	}
	h.metadata[key] = value
	return h
}

// End finishes the span and records it in the trace.
func (h *SpanHandle) End() {
	if h.trace == nil {
		return
	}
	span := Span{
		Name:       h.name,
		StartTime:  h.start,
		DurationMS: time.Since(h.start).Milliseconds(),
		Metadata:   h.metadata,
	}
	h.trace.mu.Lock()
	h.trace.spans = append(h.trace.spans, span)
	h.trace.mu.Unlock()
}
