package git

import (
	"testing"
)

func TestLoadCountries(t *testing.T) {
	countries := LoadCountries()

	if len(countries) == 0 {
		t.Fatal("LoadCountries() returned empty list")
	}

	// Check that some known countries exist
	found := map[string]bool{}
	for _, c := range countries {
		found[c] = true
	}

	for _, want := range []string{"Japan", "United States", "Brazil", "Germany", "Nigeria"} {
		if !found[want] {
			t.Errorf("expected to find %q in countries list", want)
		}
	}
}

func TestLoadCountries_NoDuplicates(t *testing.T) {
	countries := LoadCountries()
	seen := map[string]int{}
	for _, c := range countries {
		seen[c]++
	}

	for name, count := range seen {
		if count > 1 {
			t.Errorf("duplicate country: %q appears %d times", name, count)
		}
	}
}

func TestRandomCountry(t *testing.T) {
	country := RandomCountry()
	if country == "" {
		t.Error("RandomCountry() returned empty string")
	}

	// Run multiple times to verify it returns valid countries
	countries := LoadCountries()
	valid := map[string]bool{}
	for _, c := range countries {
		valid[c] = true
	}

	for i := 0; i < 10; i++ {
		c := RandomCountry()
		if !valid[c] {
			t.Errorf("RandomCountry() returned %q which is not in the countries list", c)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Japan", "japan"},
		{"United States", "united-states"},
		{"São Tomé and Príncipe", "sao-tome-and-principe"},
		{"Central African Republic", "central-african-republic"},
		{"Georgia (country)", "georgia"},
		{"Ivory Coast", "ivory-coast"},
		{"North Korea", "north-korea"},
		{"Bosnia and Herzegovina", "bosnia-and-herzegovina"},
		{"The Gambia", "the-gambia"},
		{"The Philippines", "the-philippines"},
		{"Saint-Pierre and Miquelon", "saint-pierre-and-miquelon"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
