package clock

import (
	"fmt"
	"strings"
	"time"
)

// City represents a city with its timezone.
type City struct {
	Name     string
	Timezone string
}

// DefaultCities returns the default list of world cities to display.
func DefaultCities() []City {
	return []City{
		{Name: "New York", Timezone: "America/New_York"},
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Paris", Timezone: "Europe/Paris"},
		{Name: "Dubai", Timezone: "Asia/Dubai"},
		{Name: "Mumbai", Timezone: "Asia/Kolkata"},
		{Name: "Singapore", Timezone: "Asia/Singapore"},
		{Name: "Shanghai", Timezone: "Asia/Shanghai"},
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{Name: "Sydney", Timezone: "Australia/Sydney"},
		{Name: "Auckland", Timezone: "Pacific/Auckland"},
	}
}

// CityTime holds the formatted time info for a city.
type CityTime struct {
	Name     string
	Time     string
	Date     string
	Offset   string
	IsLocal  bool
}

// GetCityTime returns the current time for a city.
func GetCityTime(city City, now time.Time) (CityTime, error) {
	loc, err := time.LoadLocation(city.Timezone)
	if err != nil {
		return CityTime{}, fmt.Errorf("load timezone %s: %w", city.Timezone, err)
	}
	t := now.In(loc)
	_, offset := t.Zone()
	hours := offset / 3600
	sign := "+"
	if hours < 0 {
		sign = "-"
		hours = -hours
	}
	return CityTime{
		Name:   city.Name,
		Time:   t.Format("15:04:05"),
		Date:   t.Format("Mon, 02 Jan"),
		Offset: fmt.Sprintf("UTC%s%d", sign, hours),
	}, nil
}

// GetLocalTime returns the current local time.
func GetLocalTime(now time.Time) CityTime {
	_, offset := now.Zone()
	hours := offset / 3600
	sign := "+"
	if hours < 0 {
		sign = "-"
		hours = -hours
	}
	zone, _ := now.Zone()
	return CityTime{
		Name:    fmt.Sprintf("Local (%s)", zone),
		Time:    now.Format("15:04:05"),
		Date:    now.Format("Mon, 02 Jan 2006"),
		Offset:  fmt.Sprintf("UTC%s%d", sign, hours),
		IsLocal: true,
	}
}

// Render produces the full terminal output string.
func Render(local CityTime, cities []CityTime) string {
	var b strings.Builder

	// Header
	b.WriteString("\033[2J\033[H") // clear screen, cursor home
	b.WriteString("\033[1;36m")
	b.WriteString("  🌍 World Time Clock\033[0m\n")
	b.WriteString("\033[90m  ─────────────────────────────────────────────\033[0m\n\n")

	// Local time (highlighted)
	b.WriteString(fmt.Sprintf("  \033[1;33m⏰ %-20s\033[0m \033[1;37m%s\033[0m  \033[90m%s  %s\033[0m\n",
		local.Name, local.Time, local.Date, local.Offset))
	b.WriteString("\n")
	b.WriteString("\033[90m  ─────────────────────────────────────────────\033[0m\n\n")

	// World cities
	for _, ct := range cities {
		b.WriteString(fmt.Sprintf("  \033[36m🕐 %-20s\033[0m \033[37m%s\033[0m  \033[90m%s  %s\033[0m\n",
			ct.Name, ct.Time, ct.Date, ct.Offset))
	}

	b.WriteString("\n\033[90m  Press Ctrl+C to exit\033[0m\n")
	return b.String()
}
