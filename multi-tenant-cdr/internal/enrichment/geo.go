package enrichment

import (
	"strings"
)

type GeoResolver struct {
	// In production, this would use a real GeoIP database or API
	// For now, we'll use a simple in-memory mapping
	countryCodes map[string]string
	regionCodes  map[string]string
	cityCodes    map[string]string
}

func NewGeoResolver() *GeoResolver {
	return &GeoResolver{
		countryCodes: map[string]string{
			"+1":  "US",
			"+44": "GB",
			"+33": "FR",
			"+49": "DE",
			"+39": "IT",
			"+34": "ES",
			"+31": "NL",
			"+41": "CH",
			"+46": "SE",
			"+47": "NO",
			"+358": "FI",
			"+81": "JP",
			"+86": "CN",
			"+91": "IN",
			"+61": "AU",
			"+55": "BR",
			"+52": "MX",
			"+7":  "RU",
		},
		regionCodes: map[string]string{
			"US": "North America",
			"GB": "Europe",
			"FR": "Europe",
			"DE": "Europe",
			"IT": "Europe",
			"ES": "Europe",
			"NL": "Europe",
			"CH": "Europe",
			"SE": "Europe",
			"NO": "Europe",
			"FI": "Europe",
			"JP": "Asia",
			"CN": "Asia",
			"IN": "Asia",
			"AU": "Oceania",
			"BR": "South America",
			"MX": "North America",
			"RU": "Europe/Asia",
		},
		cityCodes: map[string]string{
			"US": "Various",
			"GB": "London",
			"FR": "Paris",
			"DE": "Berlin",
			"IT": "Rome",
			"ES": "Madrid",
			"NL": "Amsterdam",
			"CH": "Zurich",
			"SE": "Stockholm",
			"NO": "Oslo",
			"FI": "Helsinki",
			"JP": "Tokyo",
			"CN": "Beijing",
			"IN": "Mumbai",
			"AU": "Sydney",
			"BR": "Sao Paulo",
			"MX": "Mexico City",
			"RU": "Moscow",
		},
	}
}

func (g *GeoResolver) Resolve(phoneNumber string) (*GeoLocation, error) {
	// Extract country code from phone number
	countryCode := g.extractCountryCode(phoneNumber)
	if countryCode == "" {
		return &GeoLocation{}, nil
	}

	country, ok := g.countryCodes[countryCode]
	if !ok {
		return &GeoLocation{}, nil
	}

	region, _ := g.regionCodes[country]
	city, _ := g.cityCodes[country]

	return &GeoLocation{
		Country: country,
		Region:  region,
		City:    city,
		Lat:     0,
		Lon:     0,
	}, nil
}

func (g *GeoResolver) extractCountryCode(phoneNumber string) string {
	// Remove any non-digit characters
	phoneNumber = strings.ReplaceAll(phoneNumber, "+", "")
	
	// Try to match known country codes
	for code := range g.countryCodes {
		cleanCode := strings.ReplaceAll(code, "+", "")
		if strings.HasPrefix(phoneNumber, cleanCode) {
			return code
		}
	}
	
	return ""
}
