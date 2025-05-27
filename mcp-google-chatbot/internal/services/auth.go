package services

import (
	"context"
	"fmt"
	"io/ioutil" // Will change to os.ReadFile if Go version is 1.16+

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

// NewChatService creates a new Google Chat service client with authentication
// using the provided service account credentials file.
func NewChatService(ctx context.Context, credsFilePath string) (*chat.Service, error) {
	// Read the service account key file
	// For Go 1.16+, use os.ReadFile. Assuming current environment might be older for wider compatibility.
	credsBytes, err := ioutil.ReadFile(credsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account key file '%s': %w", credsFilePath, err)
	}

	// Create credentials object with required scopes.
	// chat.ChatBotScope is "https://www.googleapis.com/auth/chat.bot"
	// This scope is generally sufficient for a bot to send messages and perform its primary functions.
	// If more specific message-related permissions are needed later, chat.ChatMessagesScope
	// ("https://www.googleapis.com/auth/chat.messages") can be added.
	config, err := google.JWTConfigFromJSON(credsBytes, chat.ChatBotScope)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT config from JSON: %w", err)
	}

	// Create an HTTP client using the JWT config's token source.
	// The oauth2.NewClient will automatically handle token generation and refresh.
	client := oauth2.NewClient(ctx, config.TokenSource(ctx))

	// Create the Chat service client using the authenticated HTTP client.
	chatService, err := chat.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create chat service: %w", err)
	}

	return chatService, nil
}

// Placeholder for MCP service logic - this file is primarily for auth.go related to Chat.
// func GetMCPData( /* ... */ ) ( /* ... */ ) {
//	// Logic to interact with MyCarrierPackets API
// }

// Placeholder for auth token refresh logic for MCP - also not directly part of NewChatService
// func RefreshMCPToken( /* ... */ ) ( /* ... */ ) {
// 	// Logic to refresh MCP API token
// }
