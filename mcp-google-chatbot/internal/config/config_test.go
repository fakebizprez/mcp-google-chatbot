package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mcp-google-chatbot/internal/config" // Adjust if your actual module path is different
	"github.com/stretchr/testify/assert"
)

// clearEnvVars unsets all environment variables that the Load function reads.
// It's important to call this to ensure a clean state for each test.
func clearEnvVars(t *testing.T) {
	vars := []string{
		"GOOGLE_APPLICATION_CREDENTIALS",
		"GOOGLE_PROJECT_ID",
		"GOOGLE_CHAT_BOT_EMAIL",
		"BEARER_TOKEN",
		"REFRESH_TOKEN",
		"TOKEN_ENDPOINT_URL",
		"PORT",
		"APP_NAME",
		"LOG_LEVEL",
		"CHAT_WEBHOOK_TOKEN",
	}
	for _, v := range vars {
		err := os.Unsetenv(v)
		if err != nil {
			// t.Logf can be used if you want to note it but not fail
			// For robustness, if an unset fails, it might indicate a problem with the test environment
			// However, t.Setenv used later will override, so this is mostly for pristine state.
			t.Logf("Warning: could not unset env var %s: %v", v, err)
		}
	}
}

func TestLoadConfig_Success(t *testing.T) {
	clearEnvVars(t) // Ensure a clean slate
	t.Cleanup(func() { clearEnvVars(t) }) // Cleanup after test

	// Set up environment variables
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/path/to/creds.json")
	t.Setenv("GOOGLE_PROJECT_ID", "test-project")
	t.Setenv("GOOGLE_CHAT_BOT_EMAIL", "bot@example.com")
	t.Setenv("BEARER_TOKEN", "test-bearer-token")
	t.Setenv("REFRESH_TOKEN", "test-refresh-token")
	t.Setenv("TOKEN_ENDPOINT_URL", "https://token.example.com")
	t.Setenv("PORT", "9090")
	t.Setenv("APP_NAME", "TestApp")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("CHAT_WEBHOOK_TOKEN", "test-webhook-token")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "/path/to/creds.json", cfg.GoogleAppCredentials)
	assert.Equal(t, "test-project", cfg.GoogleProjectID)
	assert.Equal(t, "bot@example.com", cfg.GoogleChatBotEmail)
	assert.Equal(t, "test-bearer-token", cfg.BearerTokenMCP)
	assert.Equal(t, "test-refresh-token", cfg.RefreshTokenMCP)
	assert.Equal(t, "https://token.example.com", cfg.TokenEndpointURLMCP)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "TestApp", cfg.AppName)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "test-webhook-token", cfg.ChatWebhookToken)
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	requiredVars := []struct {
		name      string
		fieldName string // For more specific error message checks if needed
	}{
		{"GOOGLE_APPLICATION_CREDENTIALS", "GoogleAppCredentials"},
		{"BEARER_TOKEN", "BearerTokenMCP"},
		{"REFRESH_TOKEN", "RefreshTokenMCP"},
		{"TOKEN_ENDPOINT_URL", "TokenEndpointURLMCP"},
		// GOOGLE_PROJECT_ID is currently not strictly required in config.Load()
	}

	baseEnv := map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
		"GOOGLE_PROJECT_ID":              "test-project",
		"BEARER_TOKEN":                   "test-bearer-token",
		"REFRESH_TOKEN":                  "test-refresh-token",
		"TOKEN_ENDPOINT_URL":             "https://token.example.com",
		"PORT":                           "8080", // Provide non-required to ensure they don't interfere
	}

	for _, tt := range requiredVars {
		t.Run(fmt.Sprintf("missing_%s", tt.name), func(t *testing.T) {
			clearEnvVars(t)
			t.Cleanup(func() { clearEnvVars(t) })

			// Set all base env vars except the one we want to test for absence
			for k, v := range baseEnv {
				if k != tt.name {
					t.Setenv(k, v)
				}
			}
			// Explicitly unset the target variable, in case t.Setenv doesn't override a prior Setenv in the same test scope.
			// os.Unsetenv is more direct for ensuring absence.
			err := os.Unsetenv(tt.name)
			assert.NoError(t, err, "Failed to unset environment variable for testing missing required")


			cfg, errLoad := config.Load()

			assert.Error(t, errLoad, "Expected an error when %s is missing", tt.name)
			assert.Nil(t, cfg, "Expected config to be nil on error")
			if errLoad != nil { // Check error message content
				assert.Contains(t, strings.ToLower(errLoad.Error()), strings.ToLower(tt.name), "Error message should mention the missing variable")
			}
		})
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnvVars(t)
	t.Cleanup(func() { clearEnvVars(t) })

	// Set only required environment variables
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/path/to/creds.json")
	// GOOGLE_PROJECT_ID is optional in current Load() logic
	// t.Setenv("GOOGLE_PROJECT_ID", "test-project")
	t.Setenv("BEARER_TOKEN", "test-bearer-token")
	t.Setenv("REFRESH_TOKEN", "test-refresh-token")
	t.Setenv("TOKEN_ENDPOINT_URL", "https://token.example.com")
	// Omit PORT, APP_NAME, LOG_LEVEL to test defaults

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "8080", cfg.Port, "Expected default port to be '8080'")
	assert.Equal(t, "mcp-google-chatbot", cfg.AppName, "Expected default app name")
	assert.Equal(t, "info", cfg.LogLevel, "Expected default log level")

	// Check that other set variables are still loaded correctly
	assert.Equal(t, "/path/to/creds.json", cfg.GoogleAppCredentials)
}

func TestLoadConfig_DotEnv(t *testing.T) {
	clearEnvVars(t)
	t.Cleanup(func() {
		clearEnvVars(t)
		_ = os.Remove(".env") // Attempt to remove .env file after test
	})

	// Create a temporary .env file
	envContent := `
GOOGLE_APPLICATION_CREDENTIALS=/path/from/dotenv.json
BEARER_TOKEN=dotenv-bearer
REFRESH_TOKEN=dotenv-refresh
TOKEN_ENDPOINT_URL=https://dotenv.example.com
PORT=7070
APP_NAME=DotEnvApp
# GOOGLE_PROJECT_ID is not set here to show it can be absent or come from actual env
`
	// Create .env file in the current directory (where the test is run)
	// The config.Load() function looks for ".env" in the process's CWD.
	// For tests, CWD is usually the package directory.
	err := os.WriteFile(".env", []byte(envContent), 0644)
	assert.NoError(t, err, "Failed to create temporary .env file")
	defer os.Remove(".env") // Ensure it's removed even if test panics

	// Set some variables in the environment to test precedence (env should override .env if both present)
	t.Setenv("APP_NAME", "EnvVarAppName") // This should override APP_NAME from .env
	t.Setenv("LOG_LEVEL", "debug")        // This is only in env

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Values from .env file
	assert.Equal(t, "/path/from/dotenv.json", cfg.GoogleAppCredentials)
	assert.Equal(t, "dotenv-bearer", cfg.BearerTokenMCP)
	assert.Equal(t, "dotenv-refresh", cfg.RefreshTokenMCP)
	assert.Equal(t, "https://dotenv.example.com", cfg.TokenEndpointURLMCP)
	assert.Equal(t, "7070", cfg.Port)

	// Value from environment variable (should override .env)
	assert.Equal(t, "EnvVarAppName", cfg.AppName)

	// Value only from environment variable (not in .env)
	assert.Equal(t, "debug", cfg.LogLevel)

	// Value not in .env and not in explicit t.Setenv for this test (should be default or empty)
	// GOOGLE_PROJECT_ID is optional and not set in .env for this test or env, so should be ""
	assert.Equal(t, "", cfg.GoogleProjectID)
	// CHAT_WEBHOOK_TOKEN is optional and not set, so should be ""
	assert.Equal(t, "", cfg.ChatWebhookToken)
}

// TestLoadConfig_DotEnv_Only tests loading when only a .env file is present
// and no overriding environment variables are set for the .env specific values.
func TestLoadConfig_DotEnv_Only(t *testing.T) {
	clearEnvVars(t)
	// Create a unique temp dir for the .env file to avoid conflicts if tests run in parallel
	// or if the CWD is not what's expected.
	// However, godotenv.Load() typically looks in CWD. For simplicity, we assume CWD is test dir.
	tempDir := t.TempDir() // Creates a temporary directory that is cleaned up
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(tempDir) // Change CWD to tempDir for this test
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalWd) // Restore original CWD
		clearEnvVars(t)          // .env is in tempDir, so it's auto-cleaned
	})

	envContent := `
GOOGLE_APPLICATION_CREDENTIALS=/dotenv/creds.json
BEARER_TOKEN=dotenv-only-bearer
REFRESH_TOKEN=dotenv-only-refresh
TOKEN_ENDPOINT_URL=https://dotenv-only.example.com
PORT=6060
APP_NAME=DotEnvOnlyApp
LOG_LEVEL=trace
GOOGLE_PROJECT_ID=dotenv-project-id
CHAT_WEBHOOK_TOKEN=dotenv-webhook-token
`
	// Create .env file in the temporary directory (which is now CWD)
	err = os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0644)
	assert.NoError(t, err, "Failed to create temporary .env file in tempDir")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, "/dotenv/creds.json", cfg.GoogleAppCredentials)
	assert.Equal(t, "dotenv-only-bearer", cfg.BearerTokenMCP)
	assert.Equal(t, "dotenv-only-refresh", cfg.RefreshTokenMCP)
	assert.Equal(t, "https://dotenv-only.example.com", cfg.TokenEndpointURLMCP)
	assert.Equal(t, "6060", cfg.Port)
	assert.Equal(t, "DotEnvOnlyApp", cfg.AppName)
	assert.Equal(t, "trace", cfg.LogLevel)
	assert.Equal(t, "dotenv-project-id", cfg.GoogleProjectID)
	assert.Equal(t, "dotenv-webhook-token", cfg.ChatWebhookToken)
}
