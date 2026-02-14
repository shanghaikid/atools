package ui

import (
	"strings"
	"testing"
)

func TestColorizeEnabled(t *testing.T) {
	SetColor(true)
	defer SetColor(true)

	got := colorize(Red, "hello")
	if !strings.Contains(got, "hello") {
		t.Errorf("colorize() does not contain original text")
	}
	if !strings.HasPrefix(got, Red) {
		t.Errorf("colorize() missing color prefix")
	}
	if !strings.HasSuffix(got, Reset) {
		t.Errorf("colorize() missing reset suffix")
	}
}

func TestColorizeDisabled(t *testing.T) {
	SetColor(false)
	defer SetColor(true)

	got := colorize(Red, "hello")
	if got != "hello" {
		t.Errorf("colorize() with color disabled = %q, want %q", got, "hello")
	}
}

func TestColorFunctions(t *testing.T) {
	SetColor(false)
	defer SetColor(true)

	tests := []struct {
		name string
		fn   func(string, ...any) string
		want string
	}{
		{name: "Boldf", fn: Boldf, want: "hello world"},
		{name: "Redf", fn: Redf, want: "hello world"},
		{name: "Greenf", fn: Greenf, want: "hello world"},
		{name: "Yellowf", fn: Yellowf, want: "hello world"},
		{name: "Bluef", fn: Bluef, want: "hello world"},
		{name: "Cyanf", fn: Cyanf, want: "hello world"},
		{name: "Dimf", fn: Dimf, want: "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("hello %s", "world")
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestColorFunctionsWithColor(t *testing.T) {
	SetColor(true)
	defer SetColor(true)

	got := Redf("error: %d", 42)
	if !strings.Contains(got, "error: 42") {
		t.Errorf("Redf() missing formatted text")
	}
	if !strings.Contains(got, Red) {
		t.Errorf("Redf() missing Red color code")
	}
}

func TestCostColor(t *testing.T) {
	SetColor(false)
	defer SetColor(true)

	tests := []struct {
		name string
		cost float64
		want string
	}{
		{name: "cheap", cost: 0.001, want: "$0.0010"},
		{name: "medium", cost: 0.5, want: "$0.5000"},
		{name: "expensive", cost: 1.5, want: "$1.5000"},
		{name: "zero", cost: 0, want: "$0.0000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CostColor(tt.cost)
			if got != tt.want {
				t.Errorf("CostColor(%f) = %q, want %q", tt.cost, got, tt.want)
			}
		})
	}
}

func TestCostColorWithColors(t *testing.T) {
	SetColor(true)
	defer SetColor(true)

	// Expensive: should use Red
	got := CostColor(2.0)
	if !strings.Contains(got, Red) {
		t.Errorf("CostColor(2.0) should contain Red color code")
	}

	// Medium: should use Yellow
	got = CostColor(0.5)
	if !strings.Contains(got, Yellow) {
		t.Errorf("CostColor(0.5) should contain Yellow color code")
	}

	// Cheap: should use Green
	got = CostColor(0.01)
	if !strings.Contains(got, Green) {
		t.Errorf("CostColor(0.01) should contain Green color code")
	}
}

func TestStatusColor(t *testing.T) {
	SetColor(false)
	defer SetColor(true)

	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "200 OK", code: 200, want: "200"},
		{name: "404 not found", code: 404, want: "404"},
		{name: "500 server error", code: 500, want: "500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusColor(tt.code)
			if got != tt.want {
				t.Errorf("StatusColor(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestStatusColorWithColors(t *testing.T) {
	SetColor(true)
	defer SetColor(true)

	// 5xx: Red
	got := StatusColor(500)
	if !strings.Contains(got, Red) {
		t.Errorf("StatusColor(500) should contain Red")
	}

	// 4xx: Yellow
	got = StatusColor(404)
	if !strings.Contains(got, Yellow) {
		t.Errorf("StatusColor(404) should contain Yellow")
	}

	// 2xx: Green
	got = StatusColor(200)
	if !strings.Contains(got, Green) {
		t.Errorf("StatusColor(200) should contain Green")
	}
}

func TestBudgetStatusColor(t *testing.T) {
	SetColor(false)
	defer SetColor(true)

	tests := []struct {
		name   string
		status string
		want   string
	}{
		{name: "daily limit", status: "DAILY LIMIT", want: "DAILY LIMIT"},
		{name: "monthly limit", status: "MONTHLY LIMIT", want: "MONTHLY LIMIT"},
		{name: "warn", status: "WARN", want: "WARN"},
		{name: "ok", status: "OK", want: "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BudgetStatusColor(tt.status)
			if got != tt.want {
				t.Errorf("BudgetStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestBudgetStatusColorWithColors(t *testing.T) {
	SetColor(true)
	defer SetColor(true)

	// DAILY LIMIT: Red
	got := BudgetStatusColor("DAILY LIMIT")
	if !strings.Contains(got, Red) {
		t.Errorf("BudgetStatusColor(DAILY LIMIT) should contain Red")
	}

	// WARN: Yellow
	got = BudgetStatusColor("WARN")
	if !strings.Contains(got, Yellow) {
		t.Errorf("BudgetStatusColor(WARN) should contain Yellow")
	}

	// OK: Green
	got = BudgetStatusColor("OK")
	if !strings.Contains(got, Green) {
		t.Errorf("BudgetStatusColor(OK) should contain Green")
	}
}
