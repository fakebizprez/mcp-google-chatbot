package models

import "time"

// Placeholder for the MyCarrierPackets API response structure
// Adjust fields based on the actual API documentation.

type CarrierRiskProfile struct {
	MCNumber         string        `json:"mc_number"`
	DOTNumber        string        `json:"dot_number"`
	CompanyName      string        `json:"company_name"`
	Address          string        `json:"address"`
	PhoneNumber      string        `json:"phone_number"`
	Email            string        `json:"email"`
	OverallRiskScore float64       `json:"overall_risk_score"` // Example: 0.0 - 100.0
	RiskSummary      string        `json:"risk_summary"`       // e.g., "Low", "Medium", "High"
	SafetyRating     *SafetyRating `json:"safety_rating,omitempty"`
	Insurance        *InsuranceInfo `json:"insurance,omitempty"`
	LastUpdated      time.Time     `json:"last_updated"`
}

type SafetyRating struct {
	RatingDate   time.Time `json:"rating_date"`
	Rating       string    `json:"rating"` // e.g., "Satisfactory", "Conditional"
	BASICSScores []BASIC   `json:"basics_scores,omitempty"`
}

type BASIC struct { // Behavior Analysis and Safety Improvement Categories
	Name      string  `json:"name"` // e.g., "Unsafe Driving", "HOS Compliance"
	Value     float64 `json:"value"` // Could be a percentile or score
	Threshold float64 `json:"threshold,omitempty"`
	Alert     bool    `json:"alert,omitempty"`
}

type InsuranceInfo struct {
	CargoLimit           float64   `json:"cargo_limit"`
	AutoLiabilityLimit   float64   `json:"auto_liability_limit"`
	PolicyExpirationDate time.Time `json:"policy_expiration_date"`
	IsActive             bool      `json:"is_active"`
}

// Placeholder for the token refresh API response
type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"` // Optional, if the refresh token also gets refreshed
	ExpiresIn    int    `json:"expires_in"`            // Seconds
}
