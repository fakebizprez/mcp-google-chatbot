package utils

import (
	"fmt"
	"strings"
	"time"

	"mcp-google-chatbot/internal/models"
	"google.golang.org/api/chat/v1"
)

const (
	defaultAvatarURL = "https://developers.google.com/chat/images/quickstart-app-avatar.png"
)

// FormatCarrierProfileToMessage creates a Google Chat message with cards from a CarrierRiskProfile.
func FormatCarrierProfileToMessage(profile *models.CarrierRiskProfile, mcNumber string) *chat.Message {
	if profile == nil {
		return &chat.Message{
			Text: fmt.Sprintf("Could not retrieve information for MC number: %s. The carrier may not exist, or an internal error occurred.", mcNumber),
		}
	}

	cardID := "carrier_profile_" + mcNumber
	var sections []*chat.GoogleAppsCardV1Section // Updated Section type

	// Section 1: Overview
	overviewWidgets := []*chat.GoogleAppsCardV1Widget{ // Updated Widget type
		newTextParagraphWidget(fmt.Sprintf("<b>Overall Risk:</b> %.1f/100 (%s)", profile.OverallRiskScore, nonEmpty(profile.RiskSummary, "N/A"))),
		newTextParagraphWidget(fmt.Sprintf("<b>Address:</b> %s<br><b>Phone:</b> %s<br><b>Email:</b> %s",
			nonEmpty(profile.Address, "N/A"),
			nonEmpty(profile.PhoneNumber, "N/A"),
			nonEmpty(profile.Email, "N/A"))),
		newKeyValueWidget("Last Updated", profile.LastUpdated.Format(time.RFC1123), "", "", nil),
	}
	sections = append(sections, &chat.GoogleAppsCardV1Section{Header: "Carrier Overview", Widgets: overviewWidgets})

	// Section 2: Safety (if available)
	if profile.SafetyRating != nil {
		safetyWidgets := []*chat.GoogleAppsCardV1Widget{
			newKeyValueWidget("Safety Rating", nonEmpty(profile.SafetyRating.Rating, "N/A"), "Rated on: "+profile.SafetyRating.RatingDate.Format("Jan 2, 2006"), "", nil),
		}
		for _, basic := range profile.SafetyRating.BASICSScores {
			icon := ""
			if basic.Alert {
				icon = "STAR"
			}
			basicText := fmt.Sprintf("%.1f", basic.Value)
			if basic.Threshold > 0 {
				basicText += fmt.Sprintf(" (Threshold: %.1f)", basic.Threshold)
			}
			safetyWidgets = append(safetyWidgets, newDecoratedTextWidget(basic.Name, basicText, icon, "", nil))
		}
		// Collapsible and UncollapsibleWidgetsCount are not direct fields anymore.
		// This might be handled by how widgets are added or a different section property.
		// For now, let's create a simple section. Advanced features might require more specific structs.
		sections = append(sections, &chat.GoogleAppsCardV1Section{Header: "Safety & Compliance", Widgets: safetyWidgets, Collapsible: true, UncollapsibleWidgetsCount: 1})
	}

	// Section 3: Insurance (if available)
	if profile.Insurance != nil {
		var insuranceStatusIcon, insuranceStatusText string
		expirationText := "Expires: " + profile.Insurance.PolicyExpirationDate.Format("Jan 2, 2006")

		if profile.Insurance.IsActive && profile.Insurance.PolicyExpirationDate.After(time.Now()) {
			insuranceStatusIcon = "CONFIRMATION_NUMBER_ICON"
			insuranceStatusText = "Active, " + expirationText
		} else {
			insuranceStatusIcon = "WARNING"
			insuranceStatusText = "Expired or Inactive, " + expirationText
		}
		insuranceWidgets := []*chat.GoogleAppsCardV1Widget{
			newKeyValueWidget("Cargo Liability", fmt.Sprintf("$%.2f", profile.Insurance.CargoLimit), "", "", nil),
			newKeyValueWidget("Auto Liability", fmt.Sprintf("$%.2f", profile.Insurance.AutoLiabilityLimit), "", "", nil),
			newDecoratedTextWidget("Policy Status", insuranceStatusText, insuranceStatusIcon, "", nil),
		}
		sections = append(sections, &chat.GoogleAppsCardV1Section{Header: "Insurance Coverage", Widgets: insuranceWidgets, Collapsible: true, UncollapsibleWidgetsCount: 1})
	}

	// Section 4: Action Buttons
	mcpProfileURL := fmt.Sprintf("https://app.mycarrierpackets.com/carriers/mc/%s", mcNumber)
	
	// Buttons are now typically within a ButtonList widget
	buttonWidget := &chat.GoogleAppsCardV1Widget{
		ButtonList: &chat.GoogleAppsCardV1ButtonList{
			Buttons: []*chat.GoogleAppsCardV1Button{
				{
					Text: "View on MyCarrierPackets",
					OnClick: &chat.GoogleAppsCardV1OnClick{ // Updated OnClick type
						OpenLink: &chat.GoogleAppsCardV1OpenLink{
							Url: mcpProfileURL,
						},
					},
				},
			},
		},
	}
	sections = append(sections, &chat.GoogleAppsCardV1Section{Widgets: []*chat.GoogleAppsCardV1Widget{buttonWidget}})

	cardHeaderTitle := "Carrier Information"
	if profile.CompanyName != "" {
		cardHeaderTitle = profile.CompanyName
	}

	// Main card structure updated to GoogleAppsCardV1Card
	card := &chat.CardWithId{
		CardId: cardID,
		Card: &chat.GoogleAppsCardV1Card{ // Updated Card type
			Header: &chat.GoogleAppsCardV1CardHeader{ // Updated Header type
				Title:    cardHeaderTitle,
				Subtitle: fmt.Sprintf("MC#: %s / DOT#: %s", nonEmpty(profile.MCNumber, "N/A"), nonEmpty(profile.DOTNumber, "N/A")),
				ImageUrl: defaultAvatarURL,
				// ImageType: "CIRCLE", // ImageType might have changed or been removed for a standard style
				ImageStyle: "IMAGE_STYLE_CIRCLE", // Common replacement for ImageType
			},
			Sections: sections,
		},
	}

	return &chat.Message{
		CardsV2: []*chat.CardWithId{card},
	}
}

// Helper functions updated to return *chat.GoogleAppsCardV1Widget

func newTextParagraphWidget(text string) *chat.GoogleAppsCardV1Widget {
	return &chat.GoogleAppsCardV1Widget{
		TextParagraph: &chat.GoogleAppsCardV1TextParagraph{Text: text},
	}
}

func newKeyValueWidget(topLabel, content, bottomLabel, iconName string, onClick *chat.GoogleAppsCardV1OnClick) *chat.GoogleAppsCardV1Widget {
	kv := &chat.GoogleAppsCardV1KeyValue{ // Updated KeyValue type
		TopLabel: topLabel,
		Content:  content,
	}
	if bottomLabel != "" {
		kv.BottomLabel = bottomLabel
	}
	if iconName != "" {
		kv.Icon = iconName
	}
	if onClick != nil {
		kv.OnClick = onClick
	}
	return &chat.GoogleAppsCardV1Widget{KeyValue: kv}
}

func newDecoratedTextWidget(topLabel, text, startIconName string, buttonText string, buttonOnClick *chat.GoogleAppsCardV1OnClick) *chat.GoogleAppsCardV1Widget {
	dt := &chat.GoogleAppsCardV1DecoratedText{ // Updated DecoratedText type
		TopLabel: topLabel,
		Text:     text,
	}
	if startIconName != "" {
		dt.StartIcon = &chat.GoogleAppsCardV1Icon{KnownIcon: startIconName} // Updated Icon type
	}

	// Button within DecoratedText also needs to be updated
	if buttonText != "" && buttonOnClick != nil {
		dt.Button = &chat.GoogleAppsCardV1Button{ // Updated Button type
			Text:    buttonText,
			OnClick: buttonOnClick,
		}
	}
	return &chat.GoogleAppsCardV1Widget{DecoratedText: dt}
}

func nonEmpty(s, substitute string) string {
	if strings.TrimSpace(s) == "" {
		return substitute
	}
	return s
}
