package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"mcp-google-chatbot/internal/config"
	"mcp-google-chatbot/internal/services" // Added
	"mcp-google-chatbot/internal/utils"    // Added

	"google.golang.org/api/chat/v1" // Already present, but confirmed
)

// HandleWebhook creates an http.HandlerFunc to process incoming webhooks from Google Chat.
// Updated signature to include mcpService and chatClient
func HandleWebhook(cfg *config.Config, mcpService *services.MCPService, chatClient *chat.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Webhook received", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		if r.Method != http.MethodPost {
			slog.Warn("Invalid method for webhook", "method", r.Method)
			http.Error(w, "Method not allowed. Only POST requests are accepted.", http.StatusMethodNotAllowed)
			return
		}

		// Webhook token verification
		if cfg.ChatWebhookToken != "" {
			authHeader := r.Header.Get("Authorization")
			expectedAuth := "Bearer " + cfg.ChatWebhookToken
			if authHeader != expectedAuth {
				slog.Warn("Webhook authorization failed", "received", authHeader)
				http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
				return
			}
			slog.Debug("Webhook token verified successfully.")
		} else {
			slog.Info("No CHAT_WEBHOOK_TOKEN configured. Skipping token verification. This is not recommended for production.")
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var event chat.Event
		if err := json.Unmarshal(bodyBytes, &event); err != nil {
			slog.Error("Failed to unmarshal event JSON", "error", err, "body_snippet", string(bodyBytes[:min(100, len(bodyBytes))]))
			http.Error(w, "Failed to parse event JSON. Ensure the request is a valid Google Chat event.", http.StatusBadRequest)
			return
		}

		slog.Info("Google Chat event successfully parsed", "type", event.Type, "space", event.Space.Name, "user", event.User.DisplayName, "event_time", event.EventTime)

		// Respond to HTTP immediately for all event types before processing and sending messages.
		// The actual message sending to Chat API will happen after this.
		defer func() {
			// Ensure that a response is always sent to the webhook.
			// If an error occurs before this point and http.Error was called,
			// this will attempt to write headers again, which is fine, http.Error sets a flag.
			// If processing is successful, this ensures the 200 OK is sent.
			// This defer might be redundant if all paths explicitly call w.WriteHeader or http.Error,
			// but it's a safeguard. For this refactor, we will send 200 OK and then process.
		}()

		switch event.Type {
		case "MESSAGE":
			slog.Info("Message event received",
				"message_name", event.Message.Name,
				"sender", event.Message.Sender.DisplayName,
				"text", event.Message.Text,
				"argument_text", event.Message.ArgumentText,
				"thread_name", event.Message.Thread.Name)

			mcNumber := ""
			commandDetected := false

			if event.Message.SlashCommand != nil {
				commandDetected = true
				slog.Info("Slash command detected", "command_id", event.Message.SlashCommand.CommandId, "args", event.Message.ArgumentText)
				mcNumber = strings.TrimSpace(event.Message.ArgumentText)
			} else if event.Message.Text != "" {
				textContent := strings.TrimSpace(event.Message.Text)
				// Remove bot mentions to simplify parsing
				if event.Message.Annotations != nil {
					for _, annotation := range event.Message.Annotations {
						if annotation.Type == "USER_MENTION" && annotation.UserMention != nil && annotation.UserMention.User.Name == cfg.GoogleChatBotEmail {
							textContent = strings.ReplaceAll(textContent, annotation.UserMention.Text, "") // Using annotation.UserMention.Text to get the exact string that was typed for the mention
							textContent = strings.TrimSpace(textContent)
							break
						}
					}
				}
				
				if strings.HasPrefix(strings.ToLower(textContent), "/carrier ") {
					commandDetected = true
					potentialMc := strings.TrimSpace(textContent[len("/carrier "):])
					if len(strings.Fields(potentialMc)) == 1 && potentialMc != "" {
						mcNumber = potentialMc
					} else {
						slog.Warn("Potential /carrier command in text, but MC number is invalid or multi-word.", "potential_mc", potentialMc)
						// mcNumber remains empty, help text will be sent.
					}
				}
			}
			
			// Immediately acknowledge the HTTP request
			w.WriteHeader(http.StatusOK)

			if mcNumber != "" {
				slog.Info("Fetching carrier profile", "mc_number", mcNumber, "space", event.Space.Name)
				profile, err := mcpService.GetCarrierRiskProfile(mcNumber)
				
				var messageToSend *chat.Message
				if err != nil {
					slog.Error("Error fetching carrier profile from MCPService", "error", err, "mc_number", mcNumber)
					messageToSend = utils.FormatCarrierProfileToMessage(nil, mcNumber) // Handles nil profile for error message
				} else {
					slog.Info("Successfully fetched carrier profile", "mc_number", mcNumber)
					messageToSend = utils.FormatCarrierProfileToMessage(profile, mcNumber)
				}

				if _, err := chatClient.Spaces.Messages.Create(event.Space.Name, messageToSend).Do(); err != nil {
					slog.Error("Failed to send carrier profile message to Google Chat", "error", err, "space", event.Space.Name)
				} else {
					slog.Info("Carrier profile message sent successfully to Google Chat", "space", event.Space.Name)
				}
			} else if commandDetected { // Command was detected but mcNumber is empty (e.g. "/carrier " with no number)
				slog.Warn("Command /carrier detected but MC number was empty or invalid.", "text", event.Message.Text)
				helpMessage := &chat.Message{Text: "The /carrier command requires an MC number. Usage: /carrier [MC_NUMBER]"}
				if _, err := chatClient.Spaces.Messages.Create(event.Space.Name, helpMessage).Do(); err != nil {
					slog.Error("Failed to send empty MC number help message to Google Chat", "error", err, "space", event.Space.Name)
				}
			} else { // Not a recognized command, or a generic message in a DM
				slog.Info("Generic message received or command not recognized", "text", event.Message.Text, "space_type", event.Space.Type)
				if event.Space.Type == "DM" || event.Message.SlashCommand == nil { // Send help for DMs or if it wasn't a failed slash command
					helpMessage := &chat.Message{Text: "To get carrier information, please use the command: /carrier [MC_NUMBER]"}
					if _, err := chatClient.Spaces.Messages.Create(event.Space.Name, helpMessage).Do(); err != nil {
						slog.Error("Failed to send generic help message to Google Chat", "error", err, "space", event.Space.Name)
					}
				}
				// If it was a message in a space that mentioned the bot but wasn't a command, no explicit help message might be needed.
				// The current logic sends help if it's a DM OR not a slash command (which covers mentioned messages that aren't commands).
			}
			return // Ensure no further writes to w

		case "ADDED_TO_SPACE":
			slog.Info("Bot added to space", "space_name", event.Space.DisplayName, "space_type", event.Space.Type, "user_who_added", event.User.DisplayName)
			
			// Acknowledge HTTP request first
			w.WriteHeader(http.StatusOK)

			welcomeText := fmt.Sprintf("Thanks for adding me to '%s'! I can help with freight carrier risk assessment. Use /carrier [MC_NUMBER] to get started.", event.Space.DisplayName)
			if event.Space.Type == "DM" {
				welcomeText = "Hi there! I can help with freight carrier risk assessment. Use /carrier [MC_NUMBER] to get started."
			}
			welcomeMsg := &chat.Message{Text: welcomeText}
			
			if _, err := chatClient.Spaces.Messages.Create(event.Space.Name, welcomeMsg).Do(); err != nil {
				slog.Error("Failed to send welcome message to Google Chat", "error", err, "space", event.Space.Name)
			} else {
				slog.Info("Welcome message sent successfully to Google Chat", "space", event.Space.Name)
			}
			return // Ensure no further writes to w

		case "REMOVED_FROM_SPACE":
			slog.Info("Bot removed from space", "space_name", event.Space.DisplayName, "user_who_removed", event.User.DisplayName)
			w.WriteHeader(http.StatusOK) // Just acknowledge
			return

		default:
			slog.Warn("Unknown or unhandled event type", "type", event.Type)
			w.WriteHeader(http.StatusOK) // Acknowledge even unhandled types
			return
		}
	}
}

// Helper function min for body snippet logging
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
