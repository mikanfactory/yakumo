package git

import (
	"encoding/csv"
	_ "embed"
	"math/rand"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//go:embed countries.csv
var countriesCSV string

// LoadCountries parses the embedded CSV and returns a list of country names.
func LoadCountries() []string {
	r := csv.NewReader(strings.NewReader(countriesCSV))
	records, err := r.ReadAll()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var countries []string
	for i, record := range records {
		if i == 0 {
			continue // skip header
		}
		if len(record) < 2 {
			continue
		}
		name := strings.TrimSpace(record[1])
		if name != "" && !seen[name] {
			seen[name] = true
			countries = append(countries, name)
		}
	}
	return countries
}

// RandomCountry returns a randomly selected country name from the CSV.
func RandomCountry() string {
	countries := LoadCountries()
	if len(countries) == 0 {
		return "unknown"
	}
	return countries[rand.Intn(len(countries))]
}

var parenthetical = regexp.MustCompile(`\s*\([^)]*\)\s*`)
var nonAlphaHyphen = regexp.MustCompile(`[^a-z0-9-]`)
var multiHyphen = regexp.MustCompile(`-{2,}`)

// Slugify converts a country name into a URL/branch-safe slug.
// Examples: "São Tomé and Príncipe" → "sao-tome-and-principe",
// "Georgia (country)" → "georgia".
func Slugify(name string) string {
	// Remove parenthetical suffixes like "(country)"
	name = parenthetical.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)

	// NFD decomposition then remove diacritical marks
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, name)
	if err != nil {
		result = name
	}

	result = strings.ToLower(result)
	result = strings.ReplaceAll(result, " ", "-")
	result = nonAlphaHyphen.ReplaceAllString(result, "")
	result = multiHyphen.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")

	return result
}
