package store

import (
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Countries maps each valid ISO 3166-1 alpha-2 country code to its English
// display name. The map is built at init time from golang.org/x/text so it
// stays in sync with the CLDR data bundled with that package.
//
// Use this map to populate country pickers and to validate country inputs.
var Countries map[string]string

func init() {
	namer := display.Regions(language.English)
	m := make(map[string]string)
	for a := rune('A'); a <= 'Z'; a++ {
		for b := rune('A'); b <= 'Z'; b++ {
			code := string([]rune{a, b})
			r, err := language.ParseRegion(code)
			if err != nil {
				continue
			}
			name := namer.Name(r)
			if name == "" || name == "Unknown Region" {
				continue
			}
			m[code] = name
		}
	}
	Countries = m
}
