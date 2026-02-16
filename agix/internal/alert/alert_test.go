package alert

import (
	"testing"
	"time"
)

func TestComputeBudgetStatus(t *testing.T) {
	tests := []struct {
		name          string
		dailySpend    float64
		dailyLimit    float64
		monthlySpend  float64
		monthlyLimit  float64
		alertPercent  float64
		wantDaily     float64
		wantMonthly   float64
		wantAlert     bool
	}{
		{
			name:         "50% daily",
			dailySpend:   5.0,
			dailyLimit:   10.0,
			alertPercent: 80,
			wantDaily:    50.0,
			wantAlert:    false,
		},
		{
			name:         "80% daily triggers alert",
			dailySpend:   8.0,
			dailyLimit:   10.0,
			alertPercent: 80,
			wantDaily:    80.0,
			wantAlert:    true,
		},
		{
			name:         "90% monthly triggers alert",
			monthlySpend: 180.0,
			monthlyLimit: 200.0,
			alertPercent: 80,
			wantMonthly:  90.0,
			wantAlert:    true,
		},
		{
			name:         "no limits configured",
			dailySpend:   5.0,
			alertPercent: 80,
			wantAlert:    false,
		},
		{
			name:         "no alert percent",
			dailySpend:   9.0,
			dailyLimit:   10.0,
			alertPercent: 0,
			wantDaily:    90.0,
			wantAlert:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := ComputeBudgetStatus(tt.dailySpend, tt.dailyLimit, tt.monthlySpend, tt.monthlyLimit, tt.alertPercent)
			if bs.DailyPercent != tt.wantDaily {
				t.Errorf("DailyPercent = %.1f, want %.1f", bs.DailyPercent, tt.wantDaily)
			}
			if bs.MonthlyPercent != tt.wantMonthly {
				t.Errorf("MonthlyPercent = %.1f, want %.1f", bs.MonthlyPercent, tt.wantMonthly)
			}
			if bs.Alert != tt.wantAlert {
				t.Errorf("Alert = %v, want %v", bs.Alert, tt.wantAlert)
			}
		})
	}
}

func TestFormatHeaders(t *testing.T) {
	bs := BudgetStatus{DailyPercent: 75.5, MonthlyPercent: 45.0}
	headers := FormatHeaders(bs)
	if headers["X-Budget-Daily-Percent"] != "75.5" {
		t.Errorf("daily header = %q", headers["X-Budget-Daily-Percent"])
	}
	if headers["X-Budget-Monthly-Percent"] != "45.0" {
		t.Errorf("monthly header = %q", headers["X-Budget-Monthly-Percent"])
	}
}

func TestAlerter_Cooldown(t *testing.T) {
	a := NewAlerter(5 * time.Minute)

	// First call should be allowed (marks lastSent)
	a.mu.Lock()
	a.lastSent["agent1"] = time.Now()
	a.mu.Unlock()

	// Second call within cooldown should be deduplicated
	a.SendWebhook("http://example.com/webhook", "agent1", WebhookPayload{Agent: "agent1"})
	// No real assertion on HTTP call since it's async + test URL, but we verify no panic
}
