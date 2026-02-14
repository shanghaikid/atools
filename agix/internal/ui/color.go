package ui

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

var colorEnabled = true

func init() {
	// Disable colors if NO_COLOR is set or stdout is not a terminal
	if os.Getenv("NO_COLOR") != "" {
		colorEnabled = false
	}
}

// SetColor enables or disables color output.
func SetColor(enabled bool) {
	colorEnabled = enabled
}

func colorize(color, s string) string {
	if !colorEnabled {
		return s
	}
	return color + s + Reset
}

func Boldf(format string, a ...any) string {
	return colorize(Bold, fmt.Sprintf(format, a...))
}

func Redf(format string, a ...any) string {
	return colorize(Red, fmt.Sprintf(format, a...))
}

func Greenf(format string, a ...any) string {
	return colorize(Green, fmt.Sprintf(format, a...))
}

func Yellowf(format string, a ...any) string {
	return colorize(Yellow, fmt.Sprintf(format, a...))
}

func Bluef(format string, a ...any) string {
	return colorize(Blue, fmt.Sprintf(format, a...))
}

func Cyanf(format string, a ...any) string {
	return colorize(Cyan, fmt.Sprintf(format, a...))
}

func Dimf(format string, a ...any) string {
	return colorize(Dim, fmt.Sprintf(format, a...))
}

// CostColor returns a color-coded cost string based on magnitude.
func CostColor(cost float64) string {
	s := fmt.Sprintf("$%.4f", cost)
	switch {
	case cost >= 1.0:
		return colorize(Red, s)
	case cost >= 0.1:
		return colorize(Yellow, s)
	default:
		return colorize(Green, s)
	}
}

// StatusColor returns a color-coded HTTP status string.
func StatusColor(code int) string {
	s := fmt.Sprintf("%d", code)
	switch {
	case code >= 500:
		return colorize(Red, s)
	case code >= 400:
		return colorize(Yellow, s)
	default:
		return colorize(Green, s)
	}
}

// BudgetStatusColor returns a color-coded budget status.
func BudgetStatusColor(status string) string {
	switch status {
	case "DAILY LIMIT", "MONTHLY LIMIT":
		return colorize(Red, status)
	case "WARN":
		return colorize(Yellow, status)
	default:
		return colorize(Green, status)
	}
}
