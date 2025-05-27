package models

import "time" // Keep existing time import

// MCPPreviewResponse is the top-level structure for the PreviewCarrier API response (an array).
type MCPPreviewResponse []MCPCarrierData

// MCPCarrierData holds all data for a single carrier from the PreviewCarrier API.
type MCPCarrierData struct {
	CompanyName           string                 `json:"CompanyName"` // Note: Case sensitivity from app.js JSON
	DotNumber             string                 `json:"DotNumber"`
	DocketNumber          string                 `json:"DocketNumber"` // This is the MC Number
	IsBlocked             bool                   `json:"IsBlocked"`
	FreightValidateStatus string                 `json:"FreightValidateStatus"` // e.g., "Review Recommended"
	RiskAssessmentDetails *RiskAssessmentDetails `json:"RiskAssessmentDetails"`
	// Add any other top-level fields from the actual API response if needed,
	// e.g., address, phone, based on what app.js might have implicitly used or what's valuable.
	// For now, focusing on data used by app.js for risk formatting.
}

// RiskAssessmentDetails contains the overall points and category-specific assessments.
type RiskAssessmentDetails struct {
	TotalPoints float64              `json:"TotalPoints"` // Assuming points can be non-integers
	Authority   *RiskCategoryDetails `json:"Authority"`
	Insurance   *RiskCategoryDetails `json:"Insurance"`
	Operation   *RiskCategoryDetails `json:"Operation"`
	Safety      *RiskCategoryDetails `json:"Safety"`
	Other       *RiskCategoryDetails `json:"Other"`
	// MyCarrierProtect will be derived from IsBlocked and FreightValidateStatus, not a direct API field here.
}

// RiskCategoryDetails holds points and infractions for a specific risk category.
type RiskCategoryDetails struct {
	TotalPoints float64      `json:"TotalPoints"`
	Infractions []Infraction `json:"Infractions"`
}

// Infraction defines the structure for individual risk infractions.
type Infraction struct {
	RuleText   string  `json:"RuleText"`
	RuleOutput string  `json:"RuleOutput"`
	Points     float64 `json:"Points"` // Assuming points can be non-integers
}

// TokenRefreshResponse can remain as is from the existing file.
type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

// Note: The original CarrierRiskProfile, SafetyRating, BASIC, InsuranceInfo structs
// from the old file are no longer directly mapped to the PreviewCarrier API.
// These can be removed or kept if they serve another purpose, but for PreviewCarrier,
// the MCPCarrierData and its sub-structs are the primary ones. For clarity,
// let's replace the old risk-related structs with the new ones.
