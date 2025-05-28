package utils_test

import (
	"fmt"
	"mcp-google-chatbot/internal/models"
	"mcp-google-chatbot/internal/utils" // Import the package being tested
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/chat/v1"
)

const (
	// Updated defaultAvatarURL constant
	defaultAvatarURL = "https://www.gstatic.com/images/icons/material/system/2x/business_center_black_24dp.png"
	testMCNumber     = "123456"
)

// New Test Functions for Risk Level Helpers
func TestGetRiskLevelText(t *testing.T) {
	testCases := []struct {
		name     string
		points   float64
		expected string
	}{
		{"Low_0", 0, "Low"},
		{"Low_124", 124, "Low"},
		{"Medium_125", 125, "Medium"},
		{"Medium_249", 249, "Medium"},
		{"ReviewRequired_250", 250, "Review Required"},
		{"ReviewRequired_999", 999, "Review Required"},
		{"Fail_1000", 1000, "Fail"},
		{"Fail_1500", 1500, "Fail"},
		{"NegativePoints_AsLow", -10, "Low"}, // Based on current formatting.go logic
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, utils.GetRiskLevelText(tc.points))
		})
	}
}

func TestGetRiskLevelEmoji(t *testing.T) {
	testCases := []struct {
		name     string
		points   float64
		expected string
	}{
		{"Green_0", 0, "🟢"},
		{"Green_124", 124, "🟢"},
		{"Yellow_125", 125, "🟡"},
		{"Yellow_249", 249, "🟡"},
		{"Orange_250", 250, "🟠"},
		{"Orange_999", 999, "🟠"},
		{"Red_1000", 1000, "🔴"},
		{"Red_1500", 1500, "🔴"},
		{"NegativePoints_AsGreen", -10, "🟢"}, // Based on current formatting.go logic
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, utils.GetRiskLevelEmoji(tc.points))
		})
	}
}

// Updated TestFormatCarrierProfileToMessage_NilProfile
func TestFormatCarrierProfileToMessage_NilProfile(t *testing.T) {
	mcNum := "MC00000"
	// Pass nil of the correct new type
	msg := utils.FormatCarrierProfileToMessage(nil, mcNum)

	assert.NotNil(t, msg)
	assert.Nil(t, msg.CardsV2, "CardsV2 should be nil for nil profile")
	expectedText := fmt.Sprintf("Could not retrieve complete information for MC number: %s.", mcNum)
	assert.Equal(t, expectedText, msg.Text)
}

// Refactored TestFormatCarrierProfileToMessage_FullData (Initial Part)
func TestFormatCarrierProfileToMessage_FullData(t *testing.T) {
	profile := &models.MCPCarrierData{
		CompanyName:  "Test Carrier LLC",
		DotNumber:    "987654",
		DocketNumber: testMCNumber, // Using const for MC#
		IsBlocked:    false,
		FreightValidateStatus: "Not Recommended", // Example value
		RiskAssessmentDetails: &models.RiskAssessmentDetails{
			TotalPoints: 75.5, // Example: Medium risk
			Authority: &models.RiskCategoryDetails{
				TotalPoints: 10,
				Infractions: []models.Infraction{
					{RuleText: "Auth Rule 1", RuleOutput: "Auth Output 1", Points: 10},
				},
			},
			Insurance: &models.RiskCategoryDetails{
				TotalPoints: 150, // Example: Medium for this category
				Infractions: []models.Infraction{
					{RuleText: "Ins Rule 1", RuleOutput: "Ins Output 1", Points: 150},
				},
			},
			// Operation, Safety, Other can be added similarly if needed for complete testing
			// For now, focusing on the Summary and overall structure
		},
	}

	msg := utils.FormatCarrierProfileToMessage(profile, testMCNumber)
	assert.NotNil(t, msg, "Message should not be nil")
	assert.NotNil(t, msg.CardsV2, "CardsV2 should not be nil for a valid profile")
	assert.Len(t, msg.CardsV2, 1, "Should have one card")

	card := msg.CardsV2[0].Card
	assert.NotNil(t, card, "Card should not be nil")

	// --- Header Assertions ---
	assert.NotNil(t, card.Header, "Card header should not be nil")
	assert.Equal(t, profile.CompanyName, card.Header.Title)
	expectedSubtitle := fmt.Sprintf("MC#: %s / DOT#: %s", profile.DocketNumber, profile.DotNumber)
	assert.Equal(t, expectedSubtitle, card.Header.Subtitle)
	assert.Equal(t, defaultAvatarURL, card.Header.ImageUrl) // Uses updated constant
	assert.Equal(t, "IMAGE_STYLE_CIRCLE", card.Header.ImageStyle)

	// --- Summary Section Assertions ---
	assert.True(t, len(card.Sections) > 0, "Should have at least one section")
	summarySection := card.Sections[0] // Assuming first section is Summary
	assert.Equal(t, "Summary", summarySection.Header)
	assert.NotEmpty(t, summarySection.Widgets, "Summary section should have widgets")

	// Verify Overall Assessment text in the summary section
	expectedOverallEmoji := utils.GetRiskLevelEmoji(profile.RiskAssessmentDetails.TotalPoints)
	expectedOverallText := utils.GetRiskLevelText(profile.RiskAssessmentDetails.TotalPoints)
	expectedOverallWidgetText := fmt.Sprintf("<b>Overall Assessment:</b> %s %s (%.0f Points)", expectedOverallEmoji, expectedOverallText, profile.RiskAssessmentDetails.TotalPoints)
	
	summaryTextParagraph := findTextParagraphWidget(summarySection.Widgets, "<b>Overall Assessment:</b>", false) // Partial match
	assert.NotNil(t, summaryTextParagraph, "Overall assessment widget not found in Summary")
	if summaryTextParagraph != nil {
		assert.Equal(t, expectedOverallWidgetText, summaryTextParagraph.Text)
	}

	// TODO: Add assertions for other sections (Authority, Insurance, MyCarrierProtect, Action Buttons)
	// This will require completing the findWidget helper functions and adding more detailed checks.
}

// --- Adapted Test Helper Functions ---

// findTextParagraphWidget searches for a TextParagraph widget containing specific content.
func findTextParagraphWidget(widgets []*chat.GoogleAppsCardV1Widget, content string, exactMatch bool) *chat.GoogleAppsCardV1TextParagraph {
	for _, w := range widgets {
		if w.TextParagraph != nil {
			if exactMatch {
				if w.TextParagraph.Text == content {
					return w.TextParagraph
				}
			} else {
				if strings.Contains(w.TextParagraph.Text, content) {
					return w.TextParagraph
				}
			}
		}
	}
	return nil
}

// findKeyValueWidget searches for a KeyValue widget.
// It can search by topLabel, content, or bottomLabel. Provide empty string for fields not to match.
func findKeyValueWidget(widgets []*chat.GoogleAppsCardV1Widget, topLabel, content, bottomLabel string) *chat.GoogleAppsCardV1KeyValue {
	for _, w := range widgets {
		if w.KeyValue != nil {
			match := true
			if topLabel != "" && w.KeyValue.TopLabel != topLabel {
				match = false
			}
			if content != "" && w.KeyValue.Content != content {
				match = false
			}
			if bottomLabel != "" && w.KeyValue.BottomLabel != bottomLabel {
				match = false
			}
			if match {
				return w.KeyValue
			}
		}
	}
	return nil
}

// findDecoratedTextWidget searches for a DecoratedText widget.
// It can search by topLabel or text content.
func findDecoratedTextWidget(widgets []*chat.GoogleAppsCardV1Widget, topLabel, text string, exactMatchText bool) *chat.GoogleAppsCardV1DecoratedText {
	for _, w := range widgets {
		if w.DecoratedText != nil {
			labelMatch := (topLabel == "" || w.DecoratedText.TopLabel == topLabel)
			textMatch := false
			if exactMatchText {
				textMatch = (text == "" || w.DecoratedText.Text == text)
			} else {
				textMatch = (text == "" || strings.Contains(w.DecoratedText.Text, text))
			}
			if labelMatch && textMatch {
				return w.DecoratedText
			}
		}
	}
	return nil
}

// findButtonListWidget searches for a ButtonList widget.
func findButtonListWidget(widgets []*chat.GoogleAppsCardV1Widget) *chat.GoogleAppsCardV1ButtonList {
	for _, w := range widgets {
		if w.ButtonList != nil {
			return w.ButtonList
		}
	}
	return nil
}
