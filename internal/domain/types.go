// Package domain contains the application types shared across adapters.
package domain

import "time"

// Listing is the normalized representation of a listing returned by Flipify.
// SourceID is deliberately opaque: it must never be parsed or constructed.
type Listing struct {
	ID                 int64      `json:"id"`
	SourceID           string     `json:"source_id"`
	Title              string     `json:"title"`
	Price              int        `json:"price"`
	Bedrooms           *float64   `json:"bedrooms,omitempty"`
	Bathrooms          *float64   `json:"bathrooms,omitempty"`
	Location           string     `json:"location,omitempty"`
	Description        string     `json:"description,omitempty"`
	ContactEmail       string     `json:"-"`
	ContactPhone       string     `json:"-"`
	PostedAt           *time.Time `json:"posted_at,omitempty"`
	URL                string     `json:"url,omitempty"`
	Images             []string   `json:"images,omitempty"`
	FraudScore         *float64   `json:"fraud_score,omitempty"`
	FraudFlags         []string   `json:"fraud_flags,omitempty"`
	FitScore           *float64   `json:"fit_score,omitempty"`
	UrgentEligible     bool       `json:"urgent_eligible"`
	EvaluationMS       *int64     `json:"fraud_evaluation_time_ms,omitempty"`
	ReverseMatches     []string   `json:"reverse_image_matches,omitempty"`
	ReverseImageSource string     `json:"reverse_image_source,omitempty"`
	NeighborhoodPrice  *int       `json:"neighborhood_avg_price,omitempty"`
	PricePercentile    *float64   `json:"price_percentile,omitempty"`
	Status             string     `json:"status"`
	UserNotes          string     `json:"user_notes,omitempty"`
	SeenAt             time.Time  `json:"seen_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type SignalResult struct {
	Name      string   `json:"name"`
	Risk      float64  `json:"risk"`
	Flags     []string `json:"flags,omitempty"`
	Hard      bool     `json:"hard"`
	Details   []string `json:"details,omitempty"`
	LatencyMS int64    `json:"latency_ms"`
}

type FraudResult struct {
	Score     float64        `json:"score"`
	Flags     []string       `json:"flags,omitempty"`
	HardFlags []string       `json:"hard_flags,omitempty"`
	Signals   []SignalResult `json:"signals"`
	FitScore  float64        `json:"fit_score"`
	Qualified bool           `json:"qualified"`
	Urgent    bool           `json:"urgent"`
	ElapsedMS int64          `json:"elapsed_ms"`
}

type Stats struct {
	SeenToday      int     `json:"seen_today"`
	QualifiedToday int     `json:"qualified_today"`
	AverageFraud   float64 `json:"average_fraud_score"`
	CostToday      float64 `json:"cost_today"`
	TotalListings  int     `json:"total_listings"`
}

type ModelEvaluation struct {
	EvalDate          time.Time `json:"eval_date"`
	Precision         float64   `json:"precision"`
	Recall            float64   `json:"recall"`
	F1                float64   `json:"f1"`
	FalsePositiveRate float64   `json:"false_positive_rate"`
	FalseNegativeRate float64   `json:"false_negative_rate"`
	TotalSamples      int       `json:"total_test_samples"`
	ModelVersion      string    `json:"model_version"`
}

type CostEntry struct {
	Service   string  `json:"service"`
	Cost      float64 `json:"cost"`
	UnitCount int     `json:"unit_count"`
	Notes     string  `json:"notes"`
}

type CostOverview struct {
	MonthlyByService map[string]float64 `json:"monthly_by_service"`
	RunningTotal     float64            `json:"running_total"`
	Processed        int                `json:"processed"`
	Qualified        int                `json:"qualified"`
	Notified         int                `json:"notified"`
}
