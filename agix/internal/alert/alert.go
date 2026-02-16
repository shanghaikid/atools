package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// BudgetStatus holds computed budget information for response headers.
type BudgetStatus struct {
	DailyPercent   float64
	MonthlyPercent float64
	Alert          bool
}

// ComputeBudgetStatus calculates the current budget utilization.
func ComputeBudgetStatus(dailySpend, dailyLimit, monthlySpend, monthlyLimit, alertPercent float64) BudgetStatus {
	bs := BudgetStatus{}

	if dailyLimit > 0 {
		bs.DailyPercent = (dailySpend / dailyLimit) * 100
	}
	if monthlyLimit > 0 {
		bs.MonthlyPercent = (monthlySpend / monthlyLimit) * 100
	}

	if alertPercent > 0 {
		if bs.DailyPercent >= alertPercent || bs.MonthlyPercent >= alertPercent {
			bs.Alert = true
		}
	}

	return bs
}

// Alerter sends webhook alerts with deduplication.
type Alerter struct {
	mu       sync.Mutex
	lastSent map[string]time.Time // agent → last alert time
	cooldown time.Duration
}

// NewAlerter creates an Alerter with the given cooldown between alerts per agent.
func NewAlerter(cooldown time.Duration) *Alerter {
	return &Alerter{
		lastSent: make(map[string]time.Time),
		cooldown: cooldown,
	}
}

// WebhookPayload is the JSON body sent to alert webhooks.
type WebhookPayload struct {
	Agent          string  `json:"agent"`
	DailySpend     float64 `json:"daily_spend_usd"`
	DailyLimit     float64 `json:"daily_limit_usd"`
	DailyPercent   float64 `json:"daily_percent"`
	MonthlySpend   float64 `json:"monthly_spend_usd"`
	MonthlyLimit   float64 `json:"monthly_limit_usd"`
	MonthlyPercent float64 `json:"monthly_percent"`
	Timestamp      string  `json:"timestamp"`
}

// SendWebhook fires a webhook alert if the cooldown has elapsed for this agent.
// The call is async (non-blocking).
func (a *Alerter) SendWebhook(url, agent string, payload WebhookPayload) {
	if url == "" {
		return
	}

	a.mu.Lock()
	if last, ok := a.lastSent[agent]; ok && time.Since(last) < a.cooldown {
		a.mu.Unlock()
		return
	}
	a.lastSent[agent] = time.Now()
	a.mu.Unlock()

	go func() {
		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("ALERT: failed to marshal webhook payload: %v", err)
			return
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("ALERT: webhook failed for %s: %v", agent, err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("ALERT: webhook returned %d for %s", resp.StatusCode, agent)
		}
	}()
}

// FormatHeaders returns budget headers to add to the response.
func FormatHeaders(bs BudgetStatus) map[string]string {
	headers := make(map[string]string)
	if bs.DailyPercent > 0 {
		headers["X-Budget-Daily-Percent"] = fmt.Sprintf("%.1f", bs.DailyPercent)
	}
	if bs.MonthlyPercent > 0 {
		headers["X-Budget-Monthly-Percent"] = fmt.Sprintf("%.1f", bs.MonthlyPercent)
	}
	return headers
}
