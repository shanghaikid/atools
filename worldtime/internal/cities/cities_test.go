package cities

import (
	"testing"
)

func TestDefaultCities(t *testing.T) {
	defaults := DefaultCities()
	if len(defaults) != 10 {
		t.Errorf("DefaultCities() returned %d cities, want 10", len(defaults))
	}

	// Verify all default cities have valid timezones
	for _, c := range defaults {
		if c.Name == "" {
			t.Error("DefaultCities() contains a city with empty name")
		}
		if c.Timezone == "" {
			t.Errorf("DefaultCities() city %q has empty timezone", c.Name)
		}
	}
}

func TestParseCities_ValidCities(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"single city", []string{"london"}, 1},
		{"multiple cities", []string{"london", "tokyo", "paris"}, 3},
		{"case insensitive", []string{"LONDON", "Tokyo", "pArIs"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCities(tt.input)
			if err != nil {
				t.Errorf("ParseCities(%v) unexpected error: %v", tt.input, err)
			}
			if len(result) != tt.want {
				t.Errorf("ParseCities(%v) returned %d cities, want %d", tt.input, len(result), tt.want)
			}
		})
	}
}

func TestParseCities_InvalidCities(t *testing.T) {
	_, err := ParseCities([]string{"atlantis"})
	if err == nil {
		t.Error("ParseCities(['atlantis']) expected error, got nil")
	}
}

func TestParseCities_MixedValidInvalid(t *testing.T) {
	_, err := ParseCities([]string{"london", "atlantis"})
	if err == nil {
		t.Error("ParseCities(['london', 'atlantis']) expected error, got nil")
	}
}

func TestParseCities_EmptyInput(t *testing.T) {
	_, err := ParseCities([]string{})
	if err == nil {
		t.Error("ParseCities([]) expected error, got nil")
	}
}

func TestParseCities_WhitespaceOnly(t *testing.T) {
	_, err := ParseCities([]string{"  ", ""})
	if err == nil {
		t.Error("ParseCities with whitespace-only input expected error, got nil")
	}
}

func TestParseCities_MultiWordCities(t *testing.T) {
	result, err := ParseCities([]string{"new york", "hong kong", "sao paulo"})
	if err != nil {
		t.Errorf("ParseCities with multi-word cities unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("ParseCities with multi-word cities returned %d, want 3", len(result))
	}
}
