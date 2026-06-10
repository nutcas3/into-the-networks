package enrichment

type FraudDetector struct {
	// In production, this would use ML models and historical data
	// For now, we'll use rule-based detection
	rules []FraudRule
}

type FraudRule struct {
	Name        string
	Description string
	Evaluate    func(*EnrichedCDR) float64
}

type FraudResult struct {
	Score        float64
	Indicators   []string
	IsSuspicious bool
}

func NewFraudDetector() *FraudDetector {
	return &FraudDetector{
		rules: []FraudRule{
			{
				Name:        "High Duration",
				Description: "Calls with unusually long duration",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					if cdr.Duration > 3600 { // > 1 hour
						return 0.3
					}
					if cdr.Duration > 1800 { // > 30 minutes
						return 0.15
					}
					return 0
				},
			},
			{
				Name:        "International Pattern",
				Description: "Frequent international calls to high-risk countries",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					if cdr.IsInternational {
						highRiskCountries := map[string]bool{
							"RU": true,
							"KP": true,
							"IR": true,
						}
						if highRiskCountries[cdr.CalleeCountry] {
							return 0.4
						}
						return 0.1
					}
					return 0
				},
			},
			{
				Name:        "Premium Rate",
				Description: "Calls to premium rate numbers",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					if cdr.IsPremium {
						return 0.2
					}
					return 0
				},
			},
			{
				Name:        "Failed Pattern",
				Description: "High failure rate pattern",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					if cdr.Status == "failed" {
						return 0.15
					}
					return 0
				},
			},
			{
				Name:        "Off-Hours",
				Description: "Calls during unusual hours",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					hour := cdr.StartTime.Hour()
					if hour < 6 || hour > 22 {
						return 0.1
					}
					return 0
				},
			},
			{
				Name:        "Short Duration",
				Description: "Very short calls (potential ping calls)",
				Evaluate: func(cdr *EnrichedCDR) float64 {
					if cdr.Duration > 0 && cdr.Duration < 5 {
						return 0.2
					}
					return 0
				},
			},
		},
	}
}

func (f *FraudDetector) Analyze(cdr *EnrichedCDR) *FraudResult {
	result := &FraudResult{
		Score:        0,
		Indicators:   []string{},
		IsSuspicious: false,
	}

	for _, rule := range f.rules {
		score := rule.Evaluate(cdr)
		if score > 0 {
			result.Score += score
			result.Indicators = append(result.Indicators, rule.Name)
		}
	}

	// Cap score at 1.0
	if result.Score > 1.0 {
		result.Score = 1.0
	}

	result.IsSuspicious = result.Score > 0.5

	return result
}

func (f *FraudDetector) AddRule(rule FraudRule) {
	f.rules = append(f.rules, rule)
}
