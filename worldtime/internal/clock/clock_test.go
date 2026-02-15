package clock

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultCities(t *testing.T) {
	cities := DefaultCities()
	if len(cities) == 0 {
		t.Fatal("expected at least one default city")
	}
	for _, c := range cities {
		if c.Name == "" || c.Timezone == "" {
			t.Errorf("city has empty name or timezone: %+v", c)
		}
	}
}

func TestGetCityTime(t *testing.T) {
	now := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		city     City
		wantTime string
	}{
		{"Shanghai", City{Name: "Shanghai", Timezone: "Asia/Shanghai"}, "20:00:00"},
		{"New York", City{Name: "New York", Timezone: "America/New_York"}, "07:00:00"},
		{"London", City{Name: "London", Timezone: "Europe/London"}, "12:00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, err := GetCityTime(tt.city, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ct.Time != tt.wantTime {
				t.Errorf("got time %s, want %s", ct.Time, tt.wantTime)
			}
			if ct.Name != tt.city.Name {
				t.Errorf("got name %s, want %s", ct.Name, tt.city.Name)
			}
		})
	}
}

func TestGetCityTimeInvalidTimezone(t *testing.T) {
	now := time.Now()
	_, err := GetCityTime(City{Name: "Nowhere", Timezone: "Invalid/Zone"}, now)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestGetLocalTime(t *testing.T) {
	now := time.Now()
	lt := GetLocalTime(now)
	if !lt.IsLocal {
		t.Error("expected IsLocal to be true")
	}
	if lt.Time == "" {
		t.Error("expected non-empty time string")
	}
}

func TestRender(t *testing.T) {
	local := CityTime{Name: "Local (CST)", Time: "20:00:00", Date: "Sun, 15 Feb 2026", Offset: "UTC+8", IsLocal: true}
	cities := []CityTime{
		{Name: "New York", Time: "07:00:00", Date: "Sun, 15 Feb", Offset: "UTC-5"},
		{Name: "London", Time: "12:00:00", Date: "Sun, 15 Feb", Offset: "UTC+0"},
	}
	output := Render(local, cities)
	if !strings.Contains(output, "World Time Clock") {
		t.Error("output missing header")
	}
	if !strings.Contains(output, "Local (CST)") {
		t.Error("output missing local time")
	}
	if !strings.Contains(output, "New York") {
		t.Error("output missing New York")
	}
}
