package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	GoogleAppCredentials string
	GoogleProjectID      string
	GoogleChatBotEmail   string

	BearerTokenMCP      string
	RefreshTokenMCP     string
	TokenEndpointURLMCP string

	Port             string
	AppName          string
	LogLevel         string
	ChatWebhookToken string // Optional
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Attempt to load .env file, but don't error if it doesn't exist
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env file not found, using environment variables directly")
	}

	cfg := &Config{
		GoogleAppCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		GoogleProjectID:      os.Getenv("GOOGLE_PROJECT_ID"),
		GoogleChatBotEmail:   os.Getenv("GOOGLE_CHAT_BOT_EMAIL"),
		BearerTokenMCP:       os.Getenv("BEARER_TOKEN"),
		RefreshTokenMCP:      os.Getenv("REFRESH_TOKEN"),
		TokenEndpointURLMCP:  os.Getenv("TOKEN_ENDPOINT_URL"),
		Port:                 os.Getenv("PORT"),
		AppName:              os.Getenv("APP_NAME"),
		LogLevel:             os.Getenv("LOG_LEVEL"),
		ChatWebhookToken:     os.Getenv("CHAT_WEBHOOK_TOKEN"),
	}

	// Basic validation for required fields
	if cfg.GoogleAppCredentials == "" {
		return nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	// GoogleProjectID might be optional depending on exact Google library usage, adjust if needed.
	// For now, let's consider it optional as per the example's commented out check.
	// if cfg.GoogleProjectID == "" {
	// 	return nil, fmt.Errorf("GOOGLE_PROJECT_ID is not set")
	// }
	if cfg.BearerTokenMCP == "" {
		return nil, fmt.Errorf("BEARER_TOKEN is not set")
	}
	if cfg.RefreshTokenMCP == "" {
		return nil, fmt.Errorf("REFRESH_TOKEN is not set")
	}
	if cfg.TokenEndpointURLMCP == "" {
		return nil, fmt.Errorf("TOKEN_ENDPOINT_URL is not set")
	}

	if cfg.Port == "" {
		log.Println("Info: PORT not set, defaulting to 8080")
		cfg.Port = "8080" // Default port
	}
	if cfg.AppName == "" {
		log.Println("Info: APP_NAME not set, defaulting to mcp-google-chatbot")
		cfg.AppName = "mcp-google-chatbot" // Default app name
	}
	if cfg.LogLevel == "" {
		log.Println("Info: LOG_LEVEL not set, defaulting to info")
		cfg.LogLevel = "info" // Default log level
	}

	return cfg, nil
}
