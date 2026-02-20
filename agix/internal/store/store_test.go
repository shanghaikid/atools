package store

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewStore(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("New() returned nil store")
	}
}

func TestNewStoreInvalidPath(t *testing.T) {
	_, err := New("/nonexistent/deeply/nested/dir/test.db")
	if err == nil {
		t.Error("New() with invalid path should return error")
	}
}

func TestInsertAndQueryRecentRequests(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{
			Timestamp:    now,
			AgentName:    "agent-1",
			Model:        "gpt-4o",
			Provider:     "openai",
			InputTokens:  1000,
			OutputTokens: 500,
			CostUSD:      0.0075,
			DurationMS:   1200,
			StatusCode:   200,
		},
		{
			Timestamp:    now.Add(-time.Minute),
			AgentName:    "agent-2",
			Model:        "claude-opus-4-6",
			Provider:     "anthropic",
			InputTokens:  2000,
			OutputTokens: 1000,
			CostUSD:      0.105,
			DurationMS:   2500,
			StatusCode:   200,
		},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	// Query all recent
	got, err := s.QueryRecentRequests(10, "")
	if err != nil {
		t.Fatalf("QueryRecentRequests() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("QueryRecentRequests() returned %d records, want 2", len(got))
	}

	// Most recent first
	if got[0].AgentName != "agent-1" {
		t.Errorf("first record agent = %q, want %q", got[0].AgentName, "agent-1")
	}
	if got[1].AgentName != "agent-2" {
		t.Errorf("second record agent = %q, want %q", got[1].AgentName, "agent-2")
	}
}

func TestQueryRecentRequestsWithAgentFilter(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	for _, agent := range []string{"agent-1", "agent-2", "agent-1"} {
		if err := s.Insert(&Record{
			Timestamp:    now,
			AgentName:    agent,
			Model:        "gpt-4o",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.001,
			DurationMS:   100,
			StatusCode:   200,
		}); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	got, err := s.QueryRecentRequests(10, "agent-1")
	if err != nil {
		t.Fatalf("QueryRecentRequests() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("QueryRecentRequests(agent-1) returned %d records, want 2", len(got))
	}
	for _, r := range got {
		if r.AgentName != "agent-1" {
			t.Errorf("filtered record agent = %q, want %q", r.AgentName, "agent-1")
		}
	}
}

func TestQueryRecentRequestsLimit(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		if err := s.Insert(&Record{
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			AgentName:    "agent",
			Model:        "gpt-4o",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.001,
			DurationMS:   100,
			StatusCode:   200,
		}); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	got, err := s.QueryRecentRequests(3, "")
	if err != nil {
		t.Fatalf("QueryRecentRequests() error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("QueryRecentRequests(limit=3) returned %d records, want 3", len(got))
	}
}

func TestQueryStats(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.0075, DurationMS: 1000, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-2", Model: "claude-opus-4-6", Provider: "anthropic", InputTokens: 2000, OutputTokens: 1000, CostUSD: 0.105, DurationMS: 2000, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 500, OutputTokens: 250, CostUSD: 0.00375, DurationMS: 800, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	since := now.Add(-time.Hour)
	until := now.Add(time.Hour)

	stats, err := s.QueryStats(since, until)
	if err != nil {
		t.Fatalf("QueryStats() error: %v", err)
	}

	if stats.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.TotalInput != 3500 {
		t.Errorf("TotalInput = %d, want 3500", stats.TotalInput)
	}
	if stats.TotalOutput != 1750 {
		t.Errorf("TotalOutput = %d, want 1750", stats.TotalOutput)
	}
	expectedCost := 0.0075 + 0.105 + 0.00375
	if math.Abs(stats.TotalCostUSD-expectedCost) > 1e-9 {
		t.Errorf("TotalCostUSD = %f, want %f", stats.TotalCostUSD, expectedCost)
	}
	if stats.UniqueModels != 2 {
		t.Errorf("UniqueModels = %d, want 2", stats.UniqueModels)
	}
	if stats.UniqueAgents != 2 {
		t.Errorf("UniqueAgents = %d, want 2", stats.UniqueAgents)
	}
}

func TestQueryStatsEmptyStore(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	stats, err := s.QueryStats(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryStats() error: %v", err)
	}

	if stats.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", stats.TotalRequests)
	}
	if stats.TotalCostUSD != 0 {
		t.Errorf("TotalCostUSD = %f, want 0", stats.TotalCostUSD)
	}
}

func TestQueryStatsByAgent(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.01, DurationMS: 1000, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 2000, OutputTokens: 1000, CostUSD: 0.02, DurationMS: 1200, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-2", Model: "gpt-4o-mini", Provider: "openai", InputTokens: 500, OutputTokens: 250, CostUSD: 0.001, DurationMS: 500, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	since := now.Add(-time.Hour)
	until := now.Add(time.Hour)

	agentStats, err := s.QueryStatsByAgent(since, until)
	if err != nil {
		t.Fatalf("QueryStatsByAgent() error: %v", err)
	}

	if len(agentStats) != 2 {
		t.Fatalf("QueryStatsByAgent() returned %d agents, want 2", len(agentStats))
	}

	// Sorted by cost DESC, so agent-1 first
	if agentStats[0].AgentName != "agent-1" {
		t.Errorf("first agent = %q, want %q", agentStats[0].AgentName, "agent-1")
	}
	if agentStats[0].Requests != 2 {
		t.Errorf("agent-1 requests = %d, want 2", agentStats[0].Requests)
	}
	if math.Abs(agentStats[0].CostUSD-0.03) > 1e-9 {
		t.Errorf("agent-1 cost = %f, want 0.03", agentStats[0].CostUSD)
	}
}

func TestQueryStatsByAgentEmptyName(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	// Insert record with empty agent name (should default to "(unknown)")
	if err := s.Insert(&Record{
		Timestamp: now, AgentName: "", Model: "gpt-4o", Provider: "openai",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.001, DurationMS: 100, StatusCode: 200,
	}); err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	agentStats, err := s.QueryStatsByAgent(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryStatsByAgent() error: %v", err)
	}

	if len(agentStats) != 1 {
		t.Fatalf("QueryStatsByAgent() returned %d agents, want 1", len(agentStats))
	}

	if agentStats[0].AgentName != "(unknown)" {
		t.Errorf("agent name = %q, want %q", agentStats[0].AgentName, "(unknown)")
	}
}

func TestQueryStatsByModel(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now, AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.01, DurationMS: 100, StatusCode: 200},
		{Timestamp: now, AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 2000, OutputTokens: 1000, CostUSD: 0.02, DurationMS: 200, StatusCode: 200},
		{Timestamp: now, AgentName: "a2", Model: "claude-opus-4-6", Provider: "anthropic", InputTokens: 500, OutputTokens: 100, CostUSD: 0.015, DurationMS: 300, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	since := now.Add(-time.Hour)
	until := now.Add(time.Hour)

	modelStats, err := s.QueryStatsByModel(since, until)
	if err != nil {
		t.Fatalf("QueryStatsByModel() error: %v", err)
	}

	if len(modelStats) != 2 {
		t.Fatalf("QueryStatsByModel() returned %d models, want 2", len(modelStats))
	}

	// Sorted by cost DESC, so gpt-4o first (0.03 total)
	if modelStats[0].Model != "gpt-4o" {
		t.Errorf("first model = %q, want %q", modelStats[0].Model, "gpt-4o")
	}
	if modelStats[0].Requests != 2 {
		t.Errorf("gpt-4o requests = %d, want 2", modelStats[0].Requests)
	}
	if modelStats[0].Provider != "openai" {
		t.Errorf("gpt-4o provider = %q, want %q", modelStats[0].Provider, "openai")
	}
}

func TestQueryDailyCosts(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)

	records := []*Record{
		{Timestamp: now, AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 0.01, DurationMS: 100, StatusCode: 200},
		{Timestamp: now, AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 200, OutputTokens: 100, CostUSD: 0.02, DurationMS: 200, StatusCode: 200},
		{Timestamp: yesterday, AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 0.005, DurationMS: 100, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	daily, err := s.QueryDailyCosts(yesterday.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryDailyCosts() error: %v", err)
	}

	if len(daily) < 1 {
		t.Fatal("QueryDailyCosts() returned no results")
	}
}

func TestQueryAgentDailySpend(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 5.00, DurationMS: 100, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 200, OutputTokens: 100, CostUSD: 3.00, DurationMS: 200, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-2", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 2.00, DurationMS: 100, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	spend, err := s.QueryAgentDailySpend("agent-1", now)
	if err != nil {
		t.Fatalf("QueryAgentDailySpend() error: %v", err)
	}

	if math.Abs(spend-8.00) > 1e-9 {
		t.Errorf("QueryAgentDailySpend(agent-1) = %f, want 8.00", spend)
	}

	// Unknown agent should return 0
	spend2, err := s.QueryAgentDailySpend("nonexistent", now)
	if err != nil {
		t.Fatalf("QueryAgentDailySpend() error: %v", err)
	}
	if spend2 != 0 {
		t.Errorf("QueryAgentDailySpend(nonexistent) = %f, want 0", spend2)
	}
}

func TestQueryAgentMonthlySpend(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 10.00, DurationMS: 100, StatusCode: 200},
		{Timestamp: now, AgentName: "agent-1", Model: "gpt-4o", Provider: "openai", InputTokens: 200, OutputTokens: 100, CostUSD: 15.00, DurationMS: 200, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	spend, err := s.QueryAgentMonthlySpend("agent-1", now.Year(), now.Month())
	if err != nil {
		t.Fatalf("QueryAgentMonthlySpend() error: %v", err)
	}

	if math.Abs(spend-25.00) > 1e-9 {
		t.Errorf("QueryAgentMonthlySpend(agent-1) = %f, want 25.00", spend)
	}
}

func TestExportCSV(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	records := []*Record{
		{Timestamp: now.Add(-time.Minute), AgentName: "a1", Model: "gpt-4o", Provider: "openai", InputTokens: 100, OutputTokens: 50, CostUSD: 0.01, DurationMS: 100, StatusCode: 200},
		{Timestamp: now, AgentName: "a2", Model: "claude-opus-4-6", Provider: "anthropic", InputTokens: 200, OutputTokens: 100, CostUSD: 0.02, DurationMS: 200, StatusCode: 200},
	}

	for _, r := range records {
		if err := s.Insert(r); err != nil {
			t.Fatalf("Insert() error: %v", err)
		}
	}

	exported, err := s.ExportCSV(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ExportCSV() error: %v", err)
	}

	if len(exported) != 2 {
		t.Fatalf("ExportCSV() returned %d records, want 2", len(exported))
	}

	// Ordered by timestamp ASC
	if exported[0].AgentName != "a1" {
		t.Errorf("first exported agent = %q, want %q", exported[0].AgentName, "a1")
	}
	if exported[1].AgentName != "a2" {
		t.Errorf("second exported agent = %q, want %q", exported[1].AgentName, "a2")
	}
}

func TestExportCSVEmptyRange(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	// Insert a record
	if err := s.Insert(&Record{
		Timestamp: now, AgentName: "a1", Model: "gpt-4o", Provider: "openai",
		InputTokens: 100, OutputTokens: 50, CostUSD: 0.01, DurationMS: 100, StatusCode: 200,
	}); err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	// Query a range that doesn't include the record
	past := now.Add(-48 * time.Hour)
	exported, err := s.ExportCSV(past.Add(-time.Hour), past)
	if err != nil {
		t.Fatalf("ExportCSV() error: %v", err)
	}

	if len(exported) != 0 {
		t.Errorf("ExportCSV() returned %d records for empty range, want 0", len(exported))
	}
}

func TestInsertRecordWithZeroValues(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	// Insert a record with all zero numeric values
	r := &Record{
		Timestamp:    now,
		AgentName:    "agent",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  0,
		OutputTokens: 0,
		CostUSD:      0,
		DurationMS:   0,
		StatusCode:   200,
	}

	if err := s.Insert(r); err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	got, err := s.QueryRecentRequests(1, "")
	if err != nil {
		t.Fatalf("QueryRecentRequests() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", got[0].InputTokens)
	}
	if got[0].OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", got[0].OutputTokens)
	}
}

func TestInsertAsync(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "async_test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	now := time.Now().UTC()
	const n = 10
	for i := 0; i < n; i++ {
		s.InsertAsync(&Record{
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			AgentName:    "async-agent",
			Model:        "gpt-4o",
			Provider:     "openai",
			InputTokens:  100,
			OutputTokens: 50,
			CostUSD:      0.001,
			DurationMS:   100,
			StatusCode:   200,
		})
	}

	// Close flushes the pending batch writes.
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Reopen and verify all records were persisted.
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() reopen error: %v", err)
	}
	defer s2.Close()

	got, err := s2.QueryRecentRequests(n+5, "async-agent")
	if err != nil {
		t.Fatalf("QueryRecentRequests() error: %v", err)
	}
	if len(got) != n {
		t.Errorf("got %d records, want %d", len(got), n)
	}
}

// BenchmarkInsertSync measures the per-call latency of synchronous Insert,
// which is what the HTTP handler pays when writes are blocking.
func BenchmarkInsertSync(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench_sync.db")
	s, err := New(dbPath)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer s.Close()

	now := time.Now().UTC()
	r := &Record{
		Timestamp:    now,
		AgentName:    "bench-agent",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.0075,
		DurationMS:   1200,
		StatusCode:   200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.Insert(r); err != nil {
			b.Fatalf("Insert() error: %v", err)
		}
	}
}

// BenchmarkInsertAsync measures the per-call latency of InsertAsync,
// which is the time the HTTP handler actually blocks on.
// Uses b.N capped to channel size to measure the pure channel-send path.
func BenchmarkInsertAsync(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench_async.db")
	s, err := New(dbPath)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}

	now := time.Now().UTC()
	r := &Record{
		Timestamp:    now,
		AgentName:    "bench-agent",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.0075,
		DurationMS:   1200,
		StatusCode:   200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.InsertAsync(r)
	}
	b.StopTimer()
	s.Close() // flush remaining
}

// BenchmarkInsertBatchThroughput measures end-to-end throughput of the batch
// writer: queue N records, close to flush, measure total wall time.
func BenchmarkInsertBatchThroughput(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench_batch.db")
	s, err := New(dbPath)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}

	now := time.Now().UTC()
	r := &Record{
		Timestamp:    now,
		AgentName:    "bench-agent",
		Model:        "gpt-4o",
		Provider:     "openai",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.0075,
		DurationMS:   1200,
		StatusCode:   200,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.InsertAsync(r)
	}
	// Flush ensures all writes complete — measures true end-to-end throughput.
	s.Close()
	b.StopTimer()
}

func TestInsertAndQueryTrace(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	spansJSON := []byte(`[{"name":"upstream","duration_ms":150,"metadata":{"status":200}}]`)
	if err := s.InsertTrace("abc123def456", "agent-1", "gpt-4o", now, spansJSON); err != nil {
		t.Fatalf("InsertTrace() error: %v", err)
	}

	tr, err := s.QueryTrace("abc123def456")
	if err != nil {
		t.Fatalf("QueryTrace() error: %v", err)
	}
	if tr == nil {
		t.Fatal("QueryTrace() returned nil")
	}
	if tr.TraceID != "abc123def456" {
		t.Errorf("trace_id = %q, want %q", tr.TraceID, "abc123def456")
	}
	if tr.AgentName != "agent-1" {
		t.Errorf("agent_name = %q, want %q", tr.AgentName, "agent-1")
	}
	if tr.Model != "gpt-4o" {
		t.Errorf("model = %q, want %q", tr.Model, "gpt-4o")
	}
}

func TestQueryTraceNotFound(t *testing.T) {
	s := newTestStore(t)

	tr, err := s.QueryTrace("nonexistent")
	if err != nil {
		t.Fatalf("QueryTrace() error: %v", err)
	}
	if tr != nil {
		t.Errorf("expected nil for nonexistent trace, got %v", tr)
	}
}

func TestQueryRecentTraces(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		agent := "agent-1"
		if i%2 == 0 {
			agent = "agent-2"
		}
		err := s.InsertTrace(
			fmt.Sprintf("trace%06d", i), agent, "gpt-4o",
			now.Add(time.Duration(i)*time.Second), []byte("[]"),
		)
		if err != nil {
			t.Fatalf("InsertTrace() error: %v", err)
		}
	}

	// All traces
	traces, err := s.QueryRecentTraces(10, "")
	if err != nil {
		t.Fatalf("QueryRecentTraces() error: %v", err)
	}
	if len(traces) != 5 {
		t.Errorf("got %d traces, want 5", len(traces))
	}

	// Filtered by agent
	traces, err = s.QueryRecentTraces(10, "agent-1")
	if err != nil {
		t.Fatalf("QueryRecentTraces() error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("got %d traces for agent-1, want 2", len(traces))
	}

	// Limited
	traces, err = s.QueryRecentTraces(2, "")
	if err != nil {
		t.Fatalf("QueryRecentTraces() error: %v", err)
	}
	if len(traces) != 2 {
		t.Errorf("got %d traces with limit=2, want 2", len(traces))
	}
}

func TestInsertTraceDuplicate(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	if err := s.InsertTrace("dup123456789", "a1", "gpt-4o", now, []byte("[]")); err != nil {
		t.Fatalf("first InsertTrace() error: %v", err)
	}
	err := s.InsertTrace("dup123456789", "a2", "gpt-4o", now, []byte("[]"))
	if err == nil {
		t.Error("expected error on duplicate trace_id, got nil")
	}
}

func TestStoreClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestDetectDialect(t *testing.T) {
	tests := []struct {
		dsn  string
		want Dialect
	}{
		{"/home/user/.agix/agix.db", DialectSQLite},
		{"~/.agix/agix.db", DialectSQLite},
		{"test.db", DialectSQLite},
		{"postgres://user:pass@localhost:5432/agix", DialectPostgres},
		{"postgresql://user:pass@localhost/agix?sslmode=disable", DialectPostgres},
		{"POSTGRES://USER@HOST/DB", DialectPostgres},
		{"PostgreSQL://user@host/db", DialectPostgres},
	}

	for _, tc := range tests {
		t.Run(tc.dsn, func(t *testing.T) {
			got := DetectDialect(tc.dsn)
			if got != tc.want {
				t.Errorf("DetectDialect(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}

func TestRebind(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		input   string
		want    string
	}{
		{"sqlite no change", DialectSQLite, "SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = ? AND b = ?"},
		{"postgres numbered", DialectPostgres, "SELECT * FROM t WHERE a = ? AND b = ?", "SELECT * FROM t WHERE a = $1 AND b = $2"},
		{"postgres single", DialectPostgres, "DELETE FROM t WHERE id = ?", "DELETE FROM t WHERE id = $1"},
		{"no placeholders", DialectPostgres, "SELECT 1", "SELECT 1"},
		{"sqlite passthrough", DialectSQLite, "INSERT INTO t VALUES (?, ?, ?)", "INSERT INTO t VALUES (?, ?, ?)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Rebind(tc.dialect, tc.input)
			if got != tc.want {
				t.Errorf("Rebind(%q, %q) = %q, want %q", tc.dialect, tc.input, got, tc.want)
			}
		})
	}
}

func TestStoreDialect(t *testing.T) {
	s := newTestStore(t)
	if s.Dialect() != DialectSQLite {
		t.Errorf("Dialect() = %q, want %q", s.Dialect(), DialectSQLite)
	}
}
