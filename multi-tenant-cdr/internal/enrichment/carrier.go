package enrichment

import (
	"strings"
)

type CarrierDatabase struct {
	// In production, this would use a real carrier database or API
	// For now, we'll use a simple in-memory mapping
	carriers map[string]CarrierInfo
}

func NewCarrierDatabase() *CarrierDatabase {
	return &CarrierDatabase{
		carriers: map[string]CarrierInfo{
			"verizon":   {Name: "Verizon", Type: "Mobile", Country: "US", IsPremium: true},
			"at&t":      {Name: "AT&T", Type: "Mobile", Country: "US", IsPremium: true},
			"t-mobile":  {Name: "T-Mobile", Type: "Mobile", Country: "US", IsPremium: true},
			"vodafone":  {Name: "Vodafone", Type: "Mobile", Country: "GB", IsPremium: true},
			"o2":        {Name: "O2", Type: "Mobile", Country: "GB", IsPremium: false},
			"orange":    {Name: "Orange", Type: "Mobile", Country: "FR", IsPremium: true},
			"telefonica": {Name: "Telefonica", Type: "Mobile", Country: "ES", IsPremium: true},
			"deutsche":  {Name: "Deutsche Telekom", Type: "Mobile", Country: "DE", IsPremium: true},
			"tim":       {Name: "TIM", Type: "Mobile", Country: "IT", IsPremium: false},
			"kpn":       {Name: "KPN", Type: "Mobile", Country: "NL", IsPremium: false},
			"swisscom":  {Name: "Swisscom", Type: "Mobile", Country: "CH", IsPremium: true},
			"telia":     {Name: "Telia", Type: "Mobile", Country: "SE", IsPremium: false},
			"telenor":   {Name: "Telenor", Type: "Mobile", Country: "NO", IsPremium: false},
			"elisa":     {Name: "Elisa", Type: "Mobile", Country: "FI", IsPremium: false},
			"ntt":       {Name: "NTT Docomo", Type: "Mobile", Country: "JP", IsPremium: true},
			"china":     {Name: "China Mobile", Type: "Mobile", Country: "CN", IsPremium: true},
			"airtel":    {Name: "Airtel", Type: "Mobile", Country: "IN", IsPremium: false},
			"telstra":   {Name: "Telstra", Type: "Mobile", Country: "AU", IsPremium: true},
			"vivo":      {Name: "Vivo", Type: "Mobile", Country: "BR", IsPremium: false},
			"amx":       {Name: "America Movil", Type: "Mobile", Country: "MX", IsPremium: true},
		},
	}
}

func (c *CarrierDatabase) Lookup(phoneNumber string) (*CarrierInfo, error) {
	// Simple heuristic: try to match carrier based on patterns
	// In production, this would use a proper carrier identification service
	
	phoneNumber = strings.ToLower(phoneNumber)
	
	// Try to match known carrier names in the number (simplified)
	for key, carrier := range c.carriers {
		if strings.Contains(phoneNumber, key) {
			return &carrier, nil
		}
	}
	
	// Default carrier based on country code
	countryCode := extractCountryCode(phoneNumber)
	switch countryCode {
	case "+1":
		return &CarrierInfo{Name: "US Carrier", Type: "Mobile", Country: "US", IsPremium: false}, nil
	case "+44":
		return &CarrierInfo{Name: "UK Carrier", Type: "Mobile", Country: "GB", IsPremium: false}, nil
	case "+33":
		return &CarrierInfo{Name: "FR Carrier", Type: "Mobile", Country: "FR", IsPremium: false}, nil
	default:
		return &CarrierInfo{Name: "Unknown Carrier", Type: "Unknown", Country: "Unknown", IsPremium: false}, nil
	}
}

func extractCountryCode(phoneNumber string) string {
	phoneNumber = strings.ReplaceAll(phoneNumber, "+", "")
	
	codes := []string{"+1", "+44", "+33", "+49", "+39", "+34", "+31", "+41", "+46", "+47", "+358", "+81", "+86", "+91", "+61", "+55", "+52", "+7"}
	
	for _, code := range codes {
		cleanCode := strings.ReplaceAll(code, "+", "")
		if strings.HasPrefix(phoneNumber, cleanCode) {
			return code
		}
	}
	
	return ""
}
