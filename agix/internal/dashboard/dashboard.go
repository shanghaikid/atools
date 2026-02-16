package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/store"
)

//go:embed static
var staticFiles embed.FS

// Dashboard serves the web dashboard and API endpoints.
type Dashboard struct {
	store   *store.Store
	cfg     *config.Config
}

// New creates a Dashboard handler.
func New(cfg *config.Config, st *store.Store) *Dashboard {
	return &Dashboard{store: st, cfg: cfg}
}

// Register adds dashboard routes to the given mux.
func (d *Dashboard) Register(mux *http.ServeMux) {
	// Serve static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", fileServer))
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
	})

	// API endpoints
	mux.HandleFunc("/api/stats", d.handleStats)
	mux.HandleFunc("/api/agents", d.handleAgents)
	mux.HandleFunc("/api/budgets", d.handleBudgets)
	mux.HandleFunc("/api/costs/daily", d.handleDailyCosts)
	mux.HandleFunc("/api/logs", d.handleLogs)
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	until := now

	stats, err := d.store.QueryStats(since, until)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleAgents(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -30)

	agents, err := d.store.QueryStatsByAgent(since, now)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

type budgetInfo struct {
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`
	DailySpend      float64 `json:"daily_spend"`
	MonthlySpend    float64 `json:"monthly_spend"`
}

func (d *Dashboard) handleBudgets(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	result := make(map[string]budgetInfo)

	for agent, budget := range d.cfg.Budgets {
		info := budgetInfo{
			DailyLimitUSD:   budget.DailyLimitUSD,
			MonthlyLimitUSD: budget.MonthlyLimitUSD,
		}

		if budget.DailyLimitUSD > 0 {
			spend, err := d.store.QueryAgentDailySpend(agent, now)
			if err == nil {
				info.DailySpend = spend
			}
		}
		if budget.MonthlyLimitUSD > 0 {
			spend, err := d.store.QueryAgentMonthlySpend(agent, now.Year(), now.Month())
			if err == nil {
				info.MonthlySpend = spend
			}
		}

		result[agent] = info
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (d *Dashboard) handleDailyCosts(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -30)

	costs, err := d.store.QueryDailyCosts(since, now)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(costs)
}

type logEntry struct {
	Timestamp    string  `json:"timestamp"`
	AgentName    string  `json:"agent_name"`
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	DurationMS   int64   `json:"duration_ms"`
	StatusCode   int     `json:"status_code"`
}

func (d *Dashboard) handleLogs(w http.ResponseWriter, r *http.Request) {
	records, err := d.store.QueryRecentRequests(50, "")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	entries := make([]logEntry, 0, len(records))
	for _, rec := range records {
		entries = append(entries, logEntry{
			Timestamp:    rec.Timestamp.Format(time.RFC3339),
			AgentName:    rec.AgentName,
			Model:        rec.Model,
			InputTokens:  rec.InputTokens,
			OutputTokens: rec.OutputTokens,
			CostUSD:      rec.CostUSD,
			DurationMS:   rec.DurationMS,
			StatusCode:   rec.StatusCode,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
