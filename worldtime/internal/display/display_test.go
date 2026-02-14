package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ryjiang/worldtime/internal/cities"
)

func TestRender_OutputContainsCityNames(t *testing.T) {
	cityList := []cities.City{
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
	}

	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	Render(&buf, cityList, now)

	output := buf.String()

	if !strings.Contains(output, "London") {
		t.Error("Render output does not contain 'London'")
	}
	if !strings.Contains(output, "Tokyo") {
		t.Error("Render output does not contain 'Tokyo'")
	}
}

func TestRender_OutputContainsTimeFormat(t *testing.T) {
	cityList := []cities.City{
		{Name: "London", Timezone: "Europe/London"},
	}

	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	Render(&buf, cityList, now)

	output := buf.String()

	// London at UTC 12:00 should show 12:00:00 (GMT in winter)
	if !strings.Contains(output, "2026-02-14") {
		t.Errorf("Render output does not contain expected date format, got:\n%s", output)
	}
	if !strings.Contains(output, "12:00:00") {
		t.Errorf("Render output does not contain expected time, got:\n%s", output)
	}
}

func TestRender_OutputContainsHeader(t *testing.T) {
	cityList := []cities.City{
		{Name: "London", Timezone: "Europe/London"},
	}

	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	Render(&buf, cityList, now)

	output := buf.String()

	if !strings.Contains(output, "World Time") {
		t.Error("Render output does not contain 'World Time' header")
	}
	if !strings.Contains(output, "Ctrl+C") {
		t.Error("Render output does not contain exit instructions")
	}
}

func TestRender_InvalidTimezone(t *testing.T) {
	cityList := []cities.City{
		{Name: "Unknown", Timezone: "Invalid/Timezone"},
	}

	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	Render(&buf, cityList, now)

	output := buf.String()

	if !strings.Contains(output, "error loading timezone") {
		t.Error("Render should show error for invalid timezone")
	}
}

func TestRender_AllDefaultCities(t *testing.T) {
	cityList := cities.DefaultCities()
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	Render(&buf, cityList, now)

	output := buf.String()

	for _, c := range cityList {
		if !strings.Contains(output, c.Name) {
			t.Errorf("Render output does not contain city %q", c.Name)
		}
	}
}
