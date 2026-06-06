package enrichment

import (
	"testing"
)

func TestGeoResolver_Resolve(t *testing.T) {
	resolver := NewGeoResolver()
	
	tests := []struct {
		phoneNumber string
		wantCountry string
	}{
		{"+14155551234", "US"},
		{"+442071234567", "GB"},
		{"+33123456789", "FR"},
		{"+49123456789", "DE"},
		{"+819012345678", "JP"},
	}
	
	for _, tt := range tests {
		t.Run(tt.phoneNumber, func(t *testing.T) {
			geo, err := resolver.Resolve(tt.phoneNumber)
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			
			if geo.Country != tt.wantCountry {
				t.Errorf("Expected country %s, got %s", tt.wantCountry, geo.Country)
			}
		})
	}
}

func TestGeoResolver_UnknownNumber(t *testing.T) {
	resolver := NewGeoResolver()
	
	geo, err := resolver.Resolve("1234567890")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	
	if geo.Country != "" {
		t.Errorf("Expected empty country for unknown number, got %s", geo.Country)
	}
}
