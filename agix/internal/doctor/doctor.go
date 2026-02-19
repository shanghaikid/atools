package doctor

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/agent-platform/agix/internal/config"
	"github.com/agent-platform/agix/internal/ui"
)

// Status represents the result of a health check.
type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
)

// Result holds the outcome of a single check.
type Result struct {
	Name    string
	Status  Status
	Message string
}

// Check is a single health check function.
type Check func(cfg *config.Config, configPath string) Result

// Run executes all checks and prints a diagnostic report.
func Run(w io.Writer, cfg *config.Config, configPath string) int {
	checks := []Check{
		CheckConfigPermissions,
		CheckAPIKeys,
		CheckBudgetSanity,
		CheckFirewallRules,
		CheckDatabase,
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.Boldf("  agix doctor"))
	fmt.Fprintln(w)

	var fails int
	for _, check := range checks {
		result := check(cfg, configPath)
		icon := statusIcon(result.Status)
		fmt.Fprintf(w, "  %s  %s\n", icon, result.Message)
		if result.Status == StatusFail {
			fails++
		}
	}

	fmt.Fprintln(w)
	if fails == 0 {
		fmt.Fprintln(w, ui.Greenf("  All checks passed!"))
	} else {
		fmt.Fprintln(w, ui.Redf("  %d check(s) failed", fails))
	}
	fmt.Fprintln(w)
	return fails
}

func statusIcon(s Status) string {
	switch s {
	case StatusPass:
		return ui.Greenf("PASS")
	case StatusWarn:
		return ui.Yellowf("WARN")
	case StatusFail:
		return ui.Redf("FAIL")
	default:
		return "????"
	}
}

// CheckConfigPermissions verifies config file is not world-readable.
func CheckConfigPermissions(cfg *config.Config, configPath string) Result {
	info, err := os.Stat(configPath)
	if err != nil {
		return Result{Name: "config_permissions", Status: StatusFail,
			Message: fmt.Sprintf("Config file: cannot stat %s: %v", configPath, err)}
	}
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		return Result{Name: "config_permissions", Status: StatusWarn,
			Message: fmt.Sprintf("Config file: %s is %o (should be 0600, contains API keys)", configPath, perm)}
	}
	return Result{Name: "config_permissions", Status: StatusPass,
		Message: fmt.Sprintf("Config file: %s permissions OK (%o)", configPath, perm)}
}

// CheckAPIKeys validates configured API keys by making lightweight requests.
func CheckAPIKeys(cfg *config.Config, _ string) Result {
	providers := []struct {
		name    string
		url     string
		headers map[string]string
	}{
		{"openai", "https://api.openai.com/v1/models", nil},
		{"anthropic", "https://api.anthropic.com/v1/models", map[string]string{"anthropic-version": "2023-06-01"}},
		{"deepseek", "https://api.deepseek.com/models", nil},
	}

	var configured, valid int
	var details []string

	for _, p := range providers {
		key, ok := cfg.Keys[p.name]
		if !ok || key == "" {
			continue
		}
		configured++

		err := validateAPIKey(p.name, p.url, key, p.headers)
		if err != nil {
			details = append(details, fmt.Sprintf("%s: %v", p.name, err))
		} else {
			valid++
			details = append(details, fmt.Sprintf("%s: valid", p.name))
		}
	}

	if configured == 0 {
		return Result{Name: "api_keys", Status: StatusWarn,
			Message: "API keys: no providers configured"}
	}

	msg := fmt.Sprintf("API keys: %d/%d valid", valid, configured)
	for _, d := range details {
		msg += fmt.Sprintf("\n         %s", d)
	}

	if valid < configured {
		return Result{Name: "api_keys", Status: StatusFail, Message: msg}
	}
	return Result{Name: "api_keys", Status: StatusPass, Message: msg}
}

func validateAPIKey(provider, url, key string, extraHeaders map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	switch provider {
	case "anthropic":
		req.Header.Set("x-api-key", key)
	default:
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid key (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
	}
	return nil
}

// CheckBudgetSanity validates budget configuration makes sense.
func CheckBudgetSanity(cfg *config.Config, _ string) Result {
	if len(cfg.Budgets) == 0 {
		return Result{Name: "budgets", Status: StatusPass,
			Message: "Budgets: none configured (OK)"}
	}

	var issues []string
	for agent, b := range cfg.Budgets {
		if b.DailyLimitUSD > 0 && b.MonthlyLimitUSD > 0 && b.DailyLimitUSD > b.MonthlyLimitUSD {
			issues = append(issues, fmt.Sprintf("%s: daily ($%.2f) > monthly ($%.2f)", agent, b.DailyLimitUSD, b.MonthlyLimitUSD))
		}
		if b.AlertAtPercent > 0 && (b.AlertAtPercent < 1 || b.AlertAtPercent > 100) {
			issues = append(issues, fmt.Sprintf("%s: alert_at_percent %.0f%% out of range [1,100]", agent, b.AlertAtPercent))
		}
	}

	if len(issues) > 0 {
		msg := fmt.Sprintf("Budgets: %d issue(s)", len(issues))
		for _, i := range issues {
			msg += fmt.Sprintf("\n         %s", i)
		}
		return Result{Name: "budgets", Status: StatusWarn, Message: msg}
	}
	return Result{Name: "budgets", Status: StatusPass,
		Message: fmt.Sprintf("Budgets: %d agent(s) configured OK", len(cfg.Budgets))}
}

// CheckFirewallRules validates firewall rule regex patterns.
func CheckFirewallRules(cfg *config.Config, _ string) Result {
	if !cfg.Firewall.Enabled {
		return Result{Name: "firewall", Status: StatusPass,
			Message: "Firewall: disabled (OK)"}
	}
	if len(cfg.Firewall.Rules) == 0 {
		return Result{Name: "firewall", Status: StatusPass,
			Message: "Firewall: enabled, no custom rules (built-in rules active)"}
	}

	var invalid []string
	for _, r := range cfg.Firewall.Rules {
		if _, err := regexp.Compile(r.Pattern); err != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", r.Name, err))
		}
		if r.Action != "block" && r.Action != "warn" && r.Action != "log" {
			invalid = append(invalid, fmt.Sprintf("%s: unknown action %q (expected block/warn/log)", r.Name, r.Action))
		}
	}

	if len(invalid) > 0 {
		msg := fmt.Sprintf("Firewall: %d invalid rule(s)", len(invalid))
		for _, i := range invalid {
			msg += fmt.Sprintf("\n         %s", i)
		}
		return Result{Name: "firewall", Status: StatusFail, Message: msg}
	}
	return Result{Name: "firewall", Status: StatusPass,
		Message: fmt.Sprintf("Firewall: %d rule(s) valid", len(cfg.Firewall.Rules))}
}

// CheckDatabase verifies SQLite database integrity.
func CheckDatabase(cfg *config.Config, _ string) Result {
	if cfg.Database == "" {
		return Result{Name: "database", Status: StatusFail,
			Message: "Database: path not configured"}
	}

	if _, err := os.Stat(cfg.Database); os.IsNotExist(err) {
		return Result{Name: "database", Status: StatusWarn,
			Message: fmt.Sprintf("Database: %s does not exist (will be created on first start)", cfg.Database)}
	}

	db, err := sql.Open("sqlite", cfg.Database)
	if err != nil {
		return Result{Name: "database", Status: StatusFail,
			Message: fmt.Sprintf("Database: cannot open %s: %v", cfg.Database, err)}
	}
	defer db.Close()

	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return Result{Name: "database", Status: StatusFail,
			Message: fmt.Sprintf("Database: integrity check failed: %v", err)}
	}
	if result != "ok" {
		return Result{Name: "database", Status: StatusFail,
			Message: fmt.Sprintf("Database: integrity check: %s", result)}
	}

	return Result{Name: "database", Status: StatusPass,
		Message: fmt.Sprintf("Database: %s integrity OK", cfg.Database)}
}
