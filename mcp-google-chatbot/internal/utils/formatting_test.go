package utils_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mcp-google-chatbot/internal/models"
	"mcp-google-chatbot/internal/utils" // Package under test
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/chat/v1"
)

const (
	testMCNumber     = "MC12345"
	defaultAvatarURL = "https://developers.google.com/chat/images/quickstart-app-avatar.png" // Must match the one in formatting.go
)

// Helper to find a KeyValue widget by its TopLabel
func findKeyValueWidget(widgets []*chat.WidgetMarkup, topLabel string) *chat.KeyValue {
	for _, w := range widgets {
		if w.KeyValue != nil && w.KeyValue.TopLabel == topLabel {
			return w.KeyValue
		}
	}
	return nil
}

// Helper to find a DecoratedText widget by its TopLabel
func findDecoratedTextWidget(widgets []*chat.WidgetMarkup, topLabel string) *chat.DecoratedText {
	for _, w := range widgets {
		if w.DecoratedText != nil && w.DecoratedText.TopLabel == topLabel {
			return w.DecoratedText
		}
	}
	return nil
}

// Helper to find a TextParagraph widget by its content (exact match or contains)
func findTextParagraphWidget(widgets []*chat.WidgetMarkup, content string, exactMatch bool) *chat.TextParagraph {
	for _, w := range widgets {
		if w.TextParagraph != nil {
			if exactMatch && w.TextParagraph.Text == content {
				return w.TextParagraph
			}
			if !exactMatch && strings.Contains(w.TextParagraph.Text, content) {
				return w.TextParagraph
			}
		}
	}
	return nil
}

func TestFormatCarrierProfileToMessage_FullData(t *testing.T) {
	now := time.Now()
	profile := &models.CarrierRiskProfile{
		MCNumber:         testMCNumber,
		DOTNumber:        "DOT98765",
		CompanyName:      "Test Carrier Inc.",
		Address:          "123 Main St, Anytown, USA",
		PhoneNumber:      "555-1234",
		Email:            "contact@testcarrier.com",
		OverallRiskScore: 75.5,
		RiskSummary:      "Medium Risk",
		LastUpdated:      now,
		SafetyRating: &models.SafetyRating{
			RatingDate: now.AddDate(0, -1, 0), // One month ago
			Rating:     "Satisfactory",
			BASICSScores: []models.BASIC{
				{Name: "Unsafe Driving", Value: 60.0, Alert: false},
				{Name: "HOS Compliance", Value: 85.0, Alert: true, Threshold: 80.0},
			},
		},
		Insurance: &models.InsuranceInfo{
			CargoLimit:           100000,
			AutoLiabilityLimit:   500000,
			PolicyExpirationDate: now.AddDate(1, 0, 0), // Expires in one year
			IsActive:             true,
		},
	}

	msg := utils.FormatCarrierProfileToMessage(profile, testMCNumber)

	assert.NotNil(t, msg)
	assert.Empty(t, msg.Text, "Expected no direct text message when a card is present")
	assert.NotNil(t, msg.CardsV2)
	assert.Len(t, msg.CardsV2, 1, "Expected one card")

	card := msg.CardsV2[0].Card
	assert.NotNil(t, card)
	assert.Equal(t, "carrier_profile_"+testMCNumber, msg.CardsV2[0].CardId)

	// Header
	assert.NotNil(t, card.Header)
	assert.Equal(t, profile.CompanyName, card.Header.Title)
	assert.Equal(t, fmt.Sprintf("MC#: %s / DOT#: %s", profile.MCNumber, profile.DOTNumber), card.Header.Subtitle)
	assert.Equal(t, defaultAvatarURL, card.Header.ImageUrl)
	assert.Equal(t, "CIRCLE", card.Header.ImageType)

	// Sections (Overview, Safety, Insurance, Buttons)
	assert.Len(t, card.Sections, 4, "Expected 4 sections for full data")

	// Section 1: Overview
	overviewSection := card.Sections[0]
	assert.Equal(t, "Carrier Overview", overviewSection.Header)
	assert.NotNil(t, findTextParagraphWidget(overviewSection.Widgets, fmt.Sprintf("<b>Overall Risk:</b> %.1f/100 (%s)", profile.OverallRiskScore, profile.RiskSummary), true))
	assert.NotNil(t, findTextParagraphWidget(overviewSection.Widgets, fmt.Sprintf("<b>Address:</b> %s<br><b>Phone:</b> %s<br><b>Email:</b> %s", profile.Address, profile.PhoneNumber, profile.Email), true))
	lastUpdatedKV := findKeyValueWidget(overviewSection.Widgets, "Last Updated")
	assert.NotNil(t, lastUpdatedKV)
	assert.Equal(t, profile.LastUpdated.Format(time.RFC1123), lastUpdatedKV.Content)

	// Section 2: Safety
	safetySection := card.Sections[1]
	assert.Equal(t, "Safety & Compliance", safetySection.Header)
	assert.True(t, safetySection.Collapsible)
	ratingKV := findKeyValueWidget(safetySection.Widgets, "Safety Rating")
	assert.NotNil(t, ratingKV)
	assert.Equal(t, profile.SafetyRating.Rating, ratingKV.Content)
	assert.Equal(t, "Rated on: "+profile.SafetyRating.RatingDate.Format("Jan 2, 2006"), ratingKV.BottomLabel)

	unsafeDrivingDT := findDecoratedTextWidget(safetySection.Widgets, "Unsafe Driving")
	assert.NotNil(t, unsafeDrivingDT)
	assert.Contains(t, unsafeDrivingDT.Text, "60.0")
	assert.Nil(t, unsafeDrivingDT.StartIcon, "Expected no icon for non-alert BASIC")

	hosComplianceDT := findDecoratedTextWidget(safetySection.Widgets, "HOS Compliance")
	assert.NotNil(t, hosComplianceDT)
	assert.Contains(t, hosComplianceDT.Text, "85.0")
	assert.Contains(t, hosComplianceDT.Text, "(Threshold: 80.0)")
	assert.NotNil(t, hosComplianceDT.StartIcon)
	assert.Equal(t, "STAR", hosComplianceDT.StartIcon.KnownIcon, "Expected STAR icon for alert BASIC")

	// Section 3: Insurance
	insuranceSection := card.Sections[2]
	assert.Equal(t, "Insurance Coverage", insuranceSection.Header)
	assert.True(t, insuranceSection.Collapsible)
	cargoKV := findKeyValueWidget(insuranceSection.Widgets, "Cargo Liability")
	assert.NotNil(t, cargoKV)
	assert.Equal(t, fmt.Sprintf("$%.2f", profile.Insurance.CargoLimit), cargoKV.Content)
	policyStatusDT := findDecoratedTextWidget(insuranceSection.Widgets, "Policy Status")
	assert.NotNil(t, policyStatusDT)
	assert.Equal(t, "CONFIRMATION_NUMBER_ICON", policyStatusDT.StartIcon.KnownIcon)
	assert.Contains(t, policyStatusDT.Text, "Active, Expires: "+profile.Insurance.PolicyExpirationDate.Format("Jan 2, 2006"))

	// Section 4: Buttons
	buttonSection := card.Sections[3]
	assert.Nil(t, buttonSection.Header, "Button section should not have a header")
	assert.Len(t, buttonSection.Widgets, 1)
	buttonList := buttonSection.Widgets[0].ButtonList
	assert.NotNil(t, buttonList)
	assert.Len(t, buttonList.Buttons, 1)
	button := buttonList.Buttons[0]
	assert.Equal(t, "View on MyCarrierPackets", button.Text)
	assert.NotNil(t, button.OnClick)
	assert.NotNil(t, button.OnClick.OpenLink)
	assert.Equal(t, fmt.Sprintf("https://app.mycarrierpackets.com/carriers/mc/%s", testMCNumber), button.OnClick.OpenLink.Url)
}

func TestFormatCarrierProfileToMessage_MinimalData(t *testing.T) {
	now := time.Now()
	profile := &models.CarrierRiskProfile{
		MCNumber:         "MC67890",
		DOTNumber:        "DOT11122",
		CompanyName:      "Minimal Carrier LLC",
		OverallRiskScore: 30.2,
		LastUpdated:      now,
		SafetyRating:     nil, // No safety data
		Insurance:        nil, // No insurance data
	}
	mcNum := "MC67890"
	msg := utils.FormatCarrierProfileToMessage(profile, mcNum)

	assert.NotNil(t, msg)
	assert.NotNil(t, msg.CardsV2)
	assert.Len(t, msg.CardsV2, 1)
	card := msg.CardsV2[0].Card
	assert.NotNil(t, card)

	// Header
	assert.Equal(t, profile.CompanyName, card.Header.Title)

	// Sections (Overview, Buttons - Safety and Insurance should be absent)
	assert.Len(t, card.Sections, 2, "Expected 2 sections for minimal data (Overview, Buttons)")

	// Section 1: Overview
	overviewSection := card.Sections[0]
	assert.Equal(t, "Carrier Overview", overviewSection.Header)
	assert.NotNil(t, findTextParagraphWidget(overviewSection.Widgets, fmt.Sprintf("<b>Overall Risk:</b> %.1f/100 (N/A)", profile.OverallRiskScore), true)) // RiskSummary is N/A

	// Section 2: Buttons (was Section 4 in full data)
	buttonSection := card.Sections[1]
	assert.Nil(t, buttonSection.Header)
	assert.Len(t, buttonSection.Widgets, 1)
	assert.NotNil(t, buttonSection.Widgets[0].ButtonList)
}

func TestFormatCarrierProfileToMessage_NilProfile(t *testing.T) {
	mcNum := "MC00000"
	msg := utils.FormatCarrierProfileToMessage(nil, mcNum)

	assert.NotNil(t, msg)
	assert.Nil(t, msg.CardsV2, "CardsV2 should be nil or empty for nil profile")
	assert.Contains(t, msg.Text, "Could not retrieve information for MC number: "+mcNum)
}

func TestFormatCarrierProfileToMessage_EmptyFields(t *testing.T) {
	now := time.Now()
	profile := &models.CarrierRiskProfile{
		MCNumber:         "MC11122",
		DOTNumber:        "DOT22233",
		CompanyName:      "Empty Fields Transport",
		Address:          "", // Empty
		PhoneNumber:      "   ", // Whitespace
		Email:            "",
		OverallRiskScore: 50.0,
		RiskSummary:      "", // Empty
		LastUpdated:      now,
	}
	mcNum := "MC11122"
	msg := utils.FormatCarrierProfileToMessage(profile, mcNum)

	assert.NotNil(t, msg)
	assert.NotNil(t, msg.CardsV2)
	assert.Len(t, msg.CardsV2, 1)
	card := msg.CardsV2[0].Card
	assert.NotNil(t, card)

	overviewSection := card.Sections[0] // Overview section
	assert.Equal(t, "Carrier Overview", overviewSection.Header)

	// Check for "N/A" substitution
	riskWidget := findTextParagraphWidget(overviewSection.Widgets, "<b>Overall Risk:</b>", false)
	assert.NotNil(t, riskWidget)
	assert.Contains(t, riskWidget.Text, "(N/A)") // RiskSummary was empty

	contactWidget := findTextParagraphWidget(overviewSection.Widgets, "<b>Address:</b>", false)
	assert.NotNil(t, contactWidget)
	assert.Contains(t, contactWidget.Text, "<b>Address:</b> N/A")
	assert.Contains(t, contactWidget.Text, "<b>Phone:</b> N/A")
	assert.Contains(t, contactWidget.Text, "<b>Email:</b> N/A")
}

func TestFormatCarrierProfileToMessage_InsuranceStatus(t *testing.T) {
	now := time.Now()
	baseProfile := &models.CarrierRiskProfile{
		MCNumber:    "MCInsTest",
		CompanyName: "Insurance Test Logistics",
		LastUpdated: now,
	}

	tests := []struct {
		name                string
		insuranceInfo       *models.InsuranceInfo
		expectedIcon        string
		expectedStatusText  string // Part of the status text to check
	}{
		{
			name: "Active and Not Expired",
			insuranceInfo: &models.InsuranceInfo{
				IsActive:             true,
				PolicyExpirationDate: now.AddDate(0, 6, 0), // Expires in 6 months
				CargoLimit:           10000, AutoLiabilityLimit: 100000,
			},
			expectedIcon:       "CONFIRMATION_NUMBER_ICON",
			expectedStatusText: "Active, Expires: " + now.AddDate(0, 6, 0).Format("Jan 2, 2006"),
		},
		{
			name: "Inactive",
			insuranceInfo: &models.InsuranceInfo{
				IsActive:             false,
				PolicyExpirationDate: now.AddDate(0, 6, 0), // Still future expiration
				CargoLimit:           10000, AutoLiabilityLimit: 100000,
			},
			expectedIcon:       "WARNING",
			expectedStatusText: "Expired or Inactive, Expires: " + now.AddDate(0, 6, 0).Format("Jan 2, 2006"),
		},
		{
			name: "Active but Expired",
			insuranceInfo: &models.InsuranceInfo{
				IsActive:             true,
				PolicyExpirationDate: now.AddDate(0, -1, 0), // Expired 1 month ago
				CargoLimit:           10000, AutoLiabilityLimit: 100000,
			},
			expectedIcon:       "WARNING",
			expectedStatusText: "Expired or Inactive, Expires: " + now.AddDate(0, -1, 0).Format("Jan 2, 2006"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := *baseProfile // Create a copy
			profile.Insurance = tt.insuranceInfo

			msg := utils.FormatCarrierProfileToMessage(&profile, profile.MCNumber)
			assert.NotNil(t, msg)
			assert.Len(t, msg.CardsV2, 1)
			card := msg.CardsV2[0].Card
			assert.NotNil(t, card)

			var insuranceSection *chat.Section
			for _, s := range card.Sections {
				if s.Header == "Insurance Coverage" {
					insuranceSection = s
					break
				}
			}
			assert.NotNil(t, insuranceSection, "Insurance section not found")

			policyStatusDT := findDecoratedTextWidget(insuranceSection.Widgets, "Policy Status")
			assert.NotNil(t, policyStatusDT)
			assert.NotNil(t, policyStatusDT.StartIcon)
			assert.Equal(t, tt.expectedIcon, policyStatusDT.StartIcon.KnownIcon)
			assert.Equal(t, tt.expectedStatusText, policyStatusDT.Text)
		})
	}
}
