package enrichment

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFraudDetector_Analyze(t *testing.T) {
	detector := NewFraudDetector()
	
	cdr := &EnrichedCDR{
		CDRID:        uuid.New(),
		TenantID:     uuid.New(),
		CallerNumber: "+14155551234",
		CalleeNumber: "+442071234567",
		StartTime:    time.Now(),
		Duration:     100,
		Status:       "completed",
	}
	
	result := detector.Analyze(cdr)
	
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Expected score between 0 and 1, got %f", result.Score)
	}
}

func TestFraudDetector_HighDuration(t *testing.T) {
	detector := NewFraudDetector()
	
	cdr := &EnrichedCDR{
		CDRID:        uuid.New(),
		TenantID:     uuid.New(),
		CallerNumber: "+14155551234",
		CalleeNumber: "+442071234567",
		StartTime:    time.Now(),
		Duration:     4000, // > 1 hour
		Status:       "completed",
	}
	
	result := detector.Analyze(cdr)
	
	if result.Score < 0.3 {
		t.Errorf("Expected score >= 0.3 for high duration, got %f", result.Score)
	}
	
	hasIndicator := false
	for _, ind := range result.Indicators {
		if ind == "High Duration" {
			hasIndicator = true
			break
		}
	}
	
	if !hasIndicator {
		t.Error("Expected 'High Duration' indicator")
	}
}

func TestFraudDetector_ShortDuration(t *testing.T) {
	detector := NewFraudDetector()
	
	cdr := &EnrichedCDR{
		CDRID:        uuid.New(),
		TenantID:     uuid.New(),
		CallerNumber: "+14155551234",
		CalleeNumber: "+442071234567",
		StartTime:    time.Now(),
		Duration:     3, // < 5 seconds
		Status:       "completed",
	}
	
	result := detector.Analyze(cdr)
	
	hasIndicator := false
	for _, ind := range result.Indicators {
		if ind == "Short Duration" {
			hasIndicator = true
			break
		}
	}
	
	if !hasIndicator {
		t.Error("Expected 'Short Duration' indicator")
	}
}

func TestFraudDetector_AddRule(t *testing.T) {
	detector := NewFraudDetector()
	
	initialRuleCount := len(detector.rules)
	
	newRule := FraudRule{
		Name:        "Test Rule",
		Description: "Test description",
		Evaluate: func(cdr *EnrichedCDR) float64 {
			return 0.5
		},
	}
	
	detector.AddRule(newRule)
	
	if len(detector.rules) != initialRuleCount+1 {
		t.Errorf("Expected rule count %d, got %d", initialRuleCount+1, len(detector.rules))
	}
}
