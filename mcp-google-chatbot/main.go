package main

import (
	"context" // Added
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"mcp-google-chatbot/internal/config"
	"mcp-google-chatbot/internal/handlers"
	"mcp-google-chatbot/internal/services" // Added
)

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		slog.Warn("Unknown log level specified, defaulting to Info", "level", levelStr)
		return slog.LevelInfo
	}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL: Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	handlerOptions := &slog.HandlerOptions{
		Level:     parseLogLevel(cfg.LogLevel),
		AddSource: true,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
	slog.SetDefault(logger)

	slog.Info("Configuration loaded successfully",
		"appName", cfg.AppName,
		"port", cfg.Port,
		"logLevel", cfg.LogLevel,
	)

	if cfg.GoogleAppCredentials == "" {
		slog.Warn("GOOGLE_APPLICATION_CREDENTIALS is not set. Google API interactions may fail.")
		// Depending on strictness, might os.Exit(1) here if chatClient is essential.
	}
	if cfg.ChatWebhookToken == "" {
		slog.Warn("CHAT_WEBHOOK_TOKEN is not set. Webhook security is disabled. This is NOT recommended for production.")
	}

	// Initialize Google Chat service
	ctx := context.Background()
	chatService, err := services.NewChatService(ctx, cfg.GoogleAppCredentials)
	if err != nil {
		slog.Error("Failed to create Google Chat service", "error", err)
		os.Exit(1) // Exit if Chat service fails to initialize
	}
	slog.Info("Google Chat service initialized successfully.")

	// Initialize MyCarrierPackets service
	mcpService := services.NewMCPService(cfg) // NewMCPService does not return an error
	slog.Info("MyCarrierPackets service initialized successfully.")

	// Register webhook handler with services
	http.HandleFunc("/webhook", handlers.HandleWebhook(cfg, mcpService, chatService))

	// Start server
	slog.Info("Starting HTTP server...", "address", fmt.Sprintf(":%s", cfg.Port))
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		slog.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}
}
