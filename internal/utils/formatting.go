package utils

import (
	"fmt"
	"mcp-google-chatbot/internal/models"
	"sort"
	"strings"

	"google.golang.org/api/chat/v1"
)

const (
	defaultAvatarURL = "https://www.gstatic.com/images/icons/material/system/2x/business_center_black_24dp.png" // A generic business avatar
)

func getRiskLevelText(points float64) string {
	if points >= 0 && points <= 124 {
		return "Low"
	} else if points >= 125 && points <= 249 {
		return "Medium"
	} else if points >= 250 && points <= 999 {
		return "Review Required"
	}
	return "Fail" // 1000+ or undefined
}

func getRiskLevelEmoji(points float64) string {
	if points >= 0 && points <= 124 {
		return "🟢" // Green
	} else if points >= 125 && points <= 249 {
		return "🟡" // Yellow
	} else if points >= 250 && points <= 999 {
		return "🟠" // Orange
	}
	return "🔴" // Red
}

// FormatCarrierProfileToMessage creates a Google Chat message with cards from MCPCarrierData.
func FormatCarrierProfileToMessage(profile *models.MCPCarrierData, mcNumber string) *chat.Message {
	if profile == nil || profile.RiskAssessmentDetails == nil {
		// This case is now primarily handled by the chat.go handler sending a simple text message.
		// However, if called directly with nil, provide a basic text message.
		return &chat.Message{
			Text: fmt.Sprintf("Could not retrieve complete information for MC number: %s.", mcNumber),
		}
	}

	cardID := "carrier_profile_" + mcNumber
	var sections []*chat.GoogleAppsCardV1Section

	// --- Card Header ---
	cardHeaderTitle := "Carrier Risk Assessment"
	if profile.CompanyName != "" {
		cardHeaderTitle = profile.CompanyName
	}
	dotNumber := "N/A"
	if profile.DotNumber != "" {
		dotNumber = profile.DotNumber
	}
	docketNumber := "N/A" // This is the MC Number
	if profile.DocketNumber != "" {
		docketNumber = profile.DocketNumber
	}

	cardHeader := &chat.GoogleAppsCardV1CardHeader{
		Title:    cardHeaderTitle,
		Subtitle: fmt.Sprintf("MC#: %s / DOT#: %s", docketNumber, dotNumber),
		ImageUrl: defaultAvatarURL,
		ImageStyle: "IMAGE_STYLE_CIRCLE",
	}

	// --- Overall Assessment Section ---
	overallPoints := profile.RiskAssessmentDetails.TotalPoints
	overallRiskText := getRiskLevelText(overallPoints)
	overallRiskEmoji := getRiskLevelEmoji(overallPoints)

	overviewWidgets := []*chat.GoogleAppsCardV1Widget{
		newTextParagraphWidget(fmt.Sprintf("<b>Overall Assessment:</b> %s %s (%.0f Points)", overallRiskEmoji, overallRiskText, overallPoints)),
	}
	sections = append(sections, &chat.GoogleAppsCardV1Section{Header: "Summary", Widgets: overviewWidgets})

	// --- Risk Categories Section ---
	// Order of categories as in app.js
	categoryOrder := []string{"Authority", "Insurance", "Operation", "Safety", "Other"}
	categoryMap := map[string]*models.RiskCategoryDetails{
		"Authority": profile.RiskAssessmentDetails.Authority,
		"Insurance": profile.RiskAssessmentDetails.Insurance,
		"Operation": profile.RiskAssessmentDetails.Operation,
		"Safety":    profile.RiskAssessmentDetails.Safety,
		"Other":     profile.RiskAssessmentDetails.Other,
	}

	for _, catName := range categoryOrder {
		category := categoryMap[catName]
		if category == nil { // Skip if category data is missing
			continue
		}

		catRiskText := getRiskLevelText(category.TotalPoints)
		catRiskEmoji := getRiskLevelEmoji(category.TotalPoints)
		
		var categoryWidgets []*chat.GoogleAppsCardV1Widget
		categoryHeaderText := fmt.Sprintf("%s %s: %.0f Points - %s", catRiskEmoji, catName, category.TotalPoints, catRiskText)
		categoryWidgets = append(categoryWidgets, newTextParagraphWidget(categoryHeaderText))

		if len(category.Infractions) > 0 {
			var infractionTexts []string
			for _, infraction := range category.Infractions {
				infractionTexts = append(infractionTexts, fmt.Sprintf("• <i>%s:</i> %s (%.0f pts)", nonEmpty(infraction.RuleText, "Infraction"), nonEmpty(infraction.RuleOutput,"Details N/A"), infraction.Points))
			}
			// Use DecoratedText for a slightly more structured look for infractions list, or TextParagraph for simplicity.
			// TextParagraph is simpler and handles multiple lines well.
			categoryWidgets = append(categoryWidgets, newTextParagraphWidget(strings.Join(infractionTexts, "<br>")))
		} else {
			categoryWidgets = append(categoryWidgets, newTextParagraphWidget("<i>No specific infractions noted.</i>"))
		}
		
		sections = append(sections, &chat.GoogleAppsCardV1Section{
			Header: fmt.Sprintf("%s Details", catName), // Section header for each category
			Collapsible: true,
			UncollapsibleWidgetsCount: 1, // Show the header text always
			Widgets: categoryWidgets,
		})
	}
	
	// --- MyCarrierProtect Section ---
	// This section is derived from IsBlocked and FreightValidateStatus
	var mcpInfractions []string
	mcpPoints := 0.0

	if profile.IsBlocked {
		mcpInfractions = append(mcpInfractions, "• MyCarrierProtect: Blocked - Carrier blocked by 3 or more companies (1000 pts)")
		mcpPoints += 1000
	}
	if profile.FreightValidateStatus == "Review Recommended" {
		mcpInfractions = append(mcpInfractions, "• FreightValidate Status: Carrier has a FreightValidate Review Recommended status (1000 pts)")
		mcpPoints += 1000 // Points are additive if both conditions met.
	}

	if len(mcpInfractions) > 0 {
		mcpRiskText := getRiskLevelText(mcpPoints) // Should be "Fail" if points >= 1000
		mcpRiskEmoji := getRiskLevelEmoji(mcpPoints)

		var mcpWidgets []*chat.GoogleAppsCardV1Widget
		mcpHeaderText := fmt.Sprintf("%s MyCarrierProtect: %.0f Points - %s", mcpRiskEmoji, mcpPoints, mcpRiskText)
		mcpWidgets = append(mcpWidgets, newTextParagraphWidget(mcpHeaderText))
		mcpWidgets = append(mcpWidgets, newTextParagraphWidget(strings.Join(mcpInfractions, "<br>")))
		
		sections = append(sections, &chat.GoogleAppsCardV1Section{
			Header: "MyCarrierProtect Details",
			Collapsible: true,
			UncollapsibleWidgetsCount: 1,
			Widgets: mcpWidgets,
		})
	}


	// --- Action Buttons Section ---
	// This can be a separate section without a header or part of the last data section.
	// For clarity, a separate section for buttons is often good.
	mcpProfileURL := fmt.Sprintf("https://app.mycarrierpackets.com/carriers/mc/%s", docketNumber) // Use actual docketNumber
	buttonWidget := &chat.GoogleAppsCardV1Widget{
		ButtonList: &chat.GoogleAppsCardV1ButtonList{
			Buttons: []*chat.GoogleAppsCardV1Button{
				{
					Text: "View Full Profile on MyCarrierPackets",
					Color: &chat.GoogleAppsCardV1Color{Red: 0.1, Green: 0.4, Blue: 0.8, Alpha: 1}, // Example blue color
					OnClick: &chat.GoogleAppsCardV1OnClick{
						OpenLink: &chat.GoogleAppsCardV1OpenLink{
							Url: mcpProfileURL,
						},
					},
				},
			},
		},
	}
	sections = append(sections, &chat.GoogleAppsCardV1Section{Widgets: []*chat.GoogleAppsCardV1Widget{buttonWidget}})


	// --- Final Card Assembly ---
	card := &chat.CardWithId{
		CardId: cardID,
		Card: &chat.GoogleAppsCardV1Card{
			Header:   cardHeader,
			Sections: sections,
		},
	}

	return &chat.Message{
		CardsV2: []*chat.CardWithId{card},
	}
}

// --- Helper functions for creating Card V2 widgets ---
// (These are from the original file and should be kept as they are useful)

func newTextParagraphWidget(text string) *chat.GoogleAppsCardV1Widget {
	return &chat.GoogleAppsCardV1Widget{
		TextParagraph: &chat.GoogleAppsCardV1TextParagraph{Text: text},
	}
}

func newKeyValueWidget(topLabel, content, bottomLabel, iconName string, onClick *chat.GoogleAppsCardV1OnClick) *chat.GoogleAppsCardV1Widget {
	kv := &chat.GoogleAppsCardV1KeyValue{
		TopLabel: topLabel,
		Content:  content,
	}
	if bottomLabel != "" {
		kv.BottomLabel = bottomLabel
	}
	// Icon mapping from string to KnownIcon might be needed if using dynamic icons
	if iconName != "" {
		kv.Icon = iconName // Assumes iconName is a valid KnownIcon string e.g. "STAR", "EMAIL"
	}
	if onClick != nil {
		kv.OnClick = onClick
	}
	return &chat.GoogleAppsCardV1Widget{KeyValue: kv}
}

// newDecoratedTextWidget creates a widget with text and an optional icon and button.
// Note: The 'iconName' should be a string from chat.KnownIcon (e.g., "AIRPLANE", "BOOKMARK").
func newDecoratedTextWidget(topLabel, text, startIconName string, buttonText string, buttonOnClick *chat.GoogleAppsCardV1OnClick) *chat.GoogleAppsCardV1Widget {
	dt := &chat.GoogleAppsCardV1DecoratedText{
		TopLabel: topLabel,
		Text:     text,
	}
	if startIconName != "" {
		dt.StartIcon = &chat.GoogleAppsCardV1Icon{KnownIcon: startIconName}
	}

	if buttonText != "" && buttonOnClick != nil {
		dt.Button = &chat.GoogleAppsCardV1Button{
			Text:    buttonText,
			OnClick: buttonOnClick,
		}
	}
	return &chat.GoogleAppsCardV1Widget{DecoratedText: dt}
}

// nonEmpty returns substitute if s is empty or whitespace, otherwise s.
func nonEmpty(s, substitute string) string {
	if strings.TrimSpace(s) == "" {
		return substitute
	}
	return s
}

// Ensure all necessary imports are present:
// import (
// 	"fmt"
// 	"mcp-google-chatbot/internal/models"
//  "sort" // if used for sorting keys, not used in current version
// 	"strings"
// 	"google.golang.org/api/chat/v1"
// )
