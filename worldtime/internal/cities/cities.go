// Package cities provides city-to-timezone mapping and parsing for the worldtime CLI.
package cities

import (
	"fmt"
	"strings"
	"time"
)

// City represents a city with its display name and IANA timezone identifier.
type City struct {
	Name     string
	Timezone string
}

// cityMap maps lowercase city names to their IANA timezone identifiers.
var cityMap = map[string]string{
	// Americas
	"new york":     "America/New_York",
	"los angeles":  "America/Los_Angeles",
	"chicago":      "America/Chicago",
	"toronto":      "America/Toronto",
	"vancouver":    "America/Vancouver",
	"mexico city":  "America/Mexico_City",
	"sao paulo":    "America/Sao_Paulo",
	"buenos aires": "America/Argentina/Buenos_Aires",
	"lima":         "America/Lima",
	"bogota":       "America/Bogota",
	// Europe
	"london":    "Europe/London",
	"paris":     "Europe/Paris",
	"berlin":    "Europe/Berlin",
	"madrid":    "Europe/Madrid",
	"rome":      "Europe/Rome",
	"amsterdam": "Europe/Amsterdam",
	"moscow":    "Europe/Moscow",
	"istanbul":  "Europe/Istanbul",
	"zurich":    "Europe/Zurich",
	"warsaw":    "Europe/Warsaw",
	// Asia
	"tokyo":      "Asia/Tokyo",
	"shanghai":   "Asia/Shanghai",
	"beijing":    "Asia/Shanghai",
	"hong kong":  "Asia/Hong_Kong",
	"singapore":  "Asia/Singapore",
	"seoul":      "Asia/Seoul",
	"mumbai":     "Asia/Kolkata",
	"delhi":      "Asia/Kolkata",
	"bangkok":    "Asia/Bangkok",
	"jakarta":    "Asia/Jakarta",
	"taipei":     "Asia/Taipei",
	// Middle East
	"dubai": "Asia/Dubai",
	"doha":  "Asia/Qatar",
	"riyadh": "Asia/Riyadh",
	// Oceania
	"sydney":    "Australia/Sydney",
	"melbourne": "Australia/Melbourne",
	"auckland":  "Pacific/Auckland",
	// Africa
	"cairo":        "Africa/Cairo",
	"johannesburg": "Africa/Johannesburg",
	"nairobi":      "Africa/Nairobi",
}

// DefaultCities returns 10 major cities from around the world.
func DefaultCities() []City {
	return []City{
		{Name: "New York", Timezone: "America/New_York"},
		{Name: "London", Timezone: "Europe/London"},
		{Name: "Tokyo", Timezone: "Asia/Tokyo"},
		{Name: "Sydney", Timezone: "Australia/Sydney"},
		{Name: "Paris", Timezone: "Europe/Paris"},
		{Name: "Dubai", Timezone: "Asia/Dubai"},
		{Name: "Singapore", Timezone: "Asia/Singapore"},
		{Name: "Hong Kong", Timezone: "Asia/Hong_Kong"},
		{Name: "Berlin", Timezone: "Europe/Berlin"},
		{Name: "São Paulo", Timezone: "America/Sao_Paulo"},
	}
}

// ParseCities takes a list of city name arguments and returns the corresponding
// City structs. It returns an error if any city name is not recognized.
// City names are matched case-insensitively.
func ParseCities(args []string) ([]City, error) {
	var cities []City
	var unknown []string

	for _, arg := range args {
		key := strings.ToLower(strings.TrimSpace(arg))
		if key == "" {
			continue
		}
		tz, ok := cityMap[key]
		if !ok {
			unknown = append(unknown, arg)
			continue
		}
		// Verify the timezone is valid
		if _, err := time.LoadLocation(tz); err != nil {
			unknown = append(unknown, arg)
			continue
		}
		// Use the original argument with proper casing for display
		displayName := titleCase(arg)
		cities = append(cities, City{Name: displayName, Timezone: tz})
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown city/cities: %s\nUse one of: %s",
			strings.Join(unknown, ", "),
			availableCities())
	}

	if len(cities) == 0 {
		return nil, fmt.Errorf("no valid cities specified")
	}

	return cities, nil
}

// availableCities returns a comma-separated list of known city names for help text.
func availableCities() string {
	names := make([]string, 0, len(cityMap))
	seen := make(map[string]bool)
	for name := range cityMap {
		// Deduplicate aliases (e.g., beijing/shanghai share a timezone)
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	// Sort is not critical for error output, but let's keep it tidy
	return strings.Join(names, ", ")
}

// titleCase converts a string to title case (first letter of each word capitalized).
func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
