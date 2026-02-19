package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-platform/agix/internal/config"
)

func TestCheckConfigPermissions(t *testing.T) {
	tests := []struct {
		name     string
		perm     os.FileMode
		wantStat Status
	}{
		{"secure 0600", 0o600, StatusPass},
		{"group readable 0640", 0o640, StatusWarn},
		{"world readable 0644", 0o644, StatusWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(f, []byte("port: 8080"), tt.perm); err != nil {
				t.Fatal(err)
			}
			r := CheckConfigPermissions(nil, f)
			if r.Status != tt.wantStat {
				t.Errorf("got status %d, want %d: %s", r.Status, tt.wantStat, r.Message)
			}
		})
	}
}

func TestCheckConfigPermissions_Missing(t *testing.T) {
	r := CheckConfigPermissions(nil, "/nonexistent/config.yaml")
	if r.Status != StatusFail {
		t.Errorf("got status %d, want StatusFail", r.Status)
	}
}

func TestCheckBudgetSanity(t *testing.T) {
	tests := []struct {
		name     string
		budgets  map[string]config.Budget
		wantStat Status
	}{
		{
			name:     "no budgets",
			budgets:  map[string]config.Budget{},
			wantStat: StatusPass,
		},
		{
			name: "valid budget",
			budgets: map[string]config.Budget{
				"agent1": {DailyLimitUSD: 10, MonthlyLimitUSD: 200, AlertAtPercent: 80},
			},
			wantStat: StatusPass,
		},
		{
			name: "daily exceeds monthly",
			budgets: map[string]config.Budget{
				"agent1": {DailyLimitUSD: 100, MonthlyLimitUSD: 50},
			},
			wantStat: StatusWarn,
		},
		{
			name: "alert percent out of range",
			budgets: map[string]config.Budget{
				"agent1": {DailyLimitUSD: 10, MonthlyLimitUSD: 200, AlertAtPercent: 150},
			},
			wantStat: StatusWarn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Budgets: tt.budgets}
			r := CheckBudgetSanity(cfg, "")
			if r.Status != tt.wantStat {
				t.Errorf("got status %d, want %d: %s", r.Status, tt.wantStat, r.Message)
			}
		})
	}
}

func TestCheckFirewallRules(t *testing.T) {
	tests := []struct {
		name     string
		fw       config.FirewallConfig
		wantStat Status
	}{
		{
			name:     "disabled",
			fw:       config.FirewallConfig{Enabled: false},
			wantStat: StatusPass,
		},
		{
			name:     "enabled no rules",
			fw:       config.FirewallConfig{Enabled: true, Rules: nil},
			wantStat: StatusPass,
		},
		{
			name: "valid rules",
			fw: config.FirewallConfig{
				Enabled: true,
				Rules: []config.FirewallRule{
					{Name: "test", Pattern: `(?i)ignore`, Action: "block"},
				},
			},
			wantStat: StatusPass,
		},
		{
			name: "invalid regex",
			fw: config.FirewallConfig{
				Enabled: true,
				Rules: []config.FirewallRule{
					{Name: "bad", Pattern: `[invalid`, Action: "block"},
				},
			},
			wantStat: StatusFail,
		},
		{
			name: "unknown action",
			fw: config.FirewallConfig{
				Enabled: true,
				Rules: []config.FirewallRule{
					{Name: "bad", Pattern: `test`, Action: "delete"},
				},
			},
			wantStat: StatusFail,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Firewall: tt.fw}
			r := CheckFirewallRules(cfg, "")
			if r.Status != tt.wantStat {
				t.Errorf("got status %d, want %d: %s", r.Status, tt.wantStat, r.Message)
			}
		})
	}
}

func TestCheckDatabase(t *testing.T) {
	t.Run("missing file warns", func(t *testing.T) {
		cfg := &config.Config{Database: filepath.Join(t.TempDir(), "nonexistent.db")}
		r := CheckDatabase(cfg, "")
		if r.Status != StatusWarn {
			t.Errorf("got status %d, want StatusWarn: %s", r.Status, r.Message)
		}
	})

	t.Run("empty path fails", func(t *testing.T) {
		cfg := &config.Config{Database: ""}
		r := CheckDatabase(cfg, "")
		if r.Status != StatusFail {
			t.Errorf("got status %d, want StatusFail: %s", r.Status, r.Message)
		}
	})
}

func TestCheckAPIKeys_NoneConfigured(t *testing.T) {
	cfg := &config.Config{Keys: map[string]string{}}
	r := CheckAPIKeys(cfg, "")
	if r.Status != StatusWarn {
		t.Errorf("got status %d, want StatusWarn: %s", r.Status, r.Message)
	}
}

func TestRun_Output(t *testing.T) {
	cfg := &config.Config{
		Keys:     map[string]string{},
		Budgets:  map[string]config.Budget{},
		Database: filepath.Join(t.TempDir(), "test.db"),
	}
	f := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(f, []byte("port: 8080"), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	fails := Run(&buf, cfg, f)

	output := buf.String()
	if len(output) == 0 {
		t.Error("expected non-empty output")
	}
	// Should have no fails (no API keys is just a warn)
	if fails != 0 {
		t.Errorf("expected 0 fails, got %d\noutput:\n%s", fails, output)
	}
}
