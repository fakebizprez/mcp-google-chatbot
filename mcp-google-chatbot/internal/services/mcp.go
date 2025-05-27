package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io" // Changed from io/ioutil
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"mcp-google-chatbot/internal/config"
	"mcp-google-chatbot/internal/models"
)

const (
	mcpAPIBaseURL         = "https://api.mycarrierpackets.com/v1" // Placeholder: Confirm actual API base URL
	maxRetries            = 1                                     // Max retries for a request (1 retry means 2 total attempts)
	requestTimeoutSeconds = 15
)

// mcNumberRegex validates MC numbers. Adjust if format is different.
var mcNumberRegex = regexp.MustCompile(`^[0-9]{2,10}$`)

// MCPService provides methods to interact with the MyCarrierPackets API.
type MCPService struct {
	httpClient       *http.Client
	config           *config.Config
	bearerToken      string     // Current active bearer token
	refreshTokenLock sync.Mutex // Protects token refresh logic
}

// NewMCPService creates a new MCPService instance.
func NewMCPService(cfg *config.Config) *MCPService {
	return &MCPService{
		httpClient: &http.Client{
			Timeout: time.Duration(requestTimeoutSeconds) * time.Second,
		},
		config:      cfg,
		bearerToken: cfg.BearerTokenMCP, // Initial bearer token from config
	}
}

// NewMCPServiceWithClient creates a new MCPService instance with a custom HTTP client.
// This is primarily for testing purposes.
func NewMCPServiceWithClient(cfg *config.Config, client *http.Client) *MCPService {
	return &MCPService{
		httpClient:       client,
		config:           cfg,
		bearerToken:      cfg.BearerTokenMCP, // Initial bearer token from config
		// refreshTokenLock: sync.Mutex{}, // refreshTokenLock is not pointer, so it's initialized by zero value
	}
}

// GetCarrierRiskProfile fetches the risk profile for a given MC number.
func (s *MCPService) GetCarrierRiskProfile(mcNumber string) (*models.CarrierRiskProfile, error) {
	if !mcNumberRegex.MatchString(mcNumber) {
		slog.Warn("Invalid MC number format provided", "mc_number", mcNumber)
		return nil, fmt.Errorf("invalid MC number format: '%s'. Must be 2-10 digits", mcNumber)
	}

	// Construct the API URL. This is a placeholder; replace with the actual endpoint.
	apiURL := fmt.Sprintf("%s/carrier/%s/risk-profile", mcpAPIBaseURL, mcNumber) // Example endpoint
	slog.Debug("Constructed MCP API URL", "url", apiURL)

	respBytes, err := s.doRequestWithAuth(http.MethodGet, apiURL, nil)
	if err != nil {
		// Error is already logged by doRequestWithAuth or refreshAuthToken
		return nil, fmt.Errorf("failed to get carrier risk profile for MC %s: %w", mcNumber, err)
	}

	var profile models.CarrierRiskProfile
	if err := json.Unmarshal(respBytes, &profile); err != nil {
		slog.Error("Failed to unmarshal MCP API response into CarrierRiskProfile", "error", err, "mc_number", mcNumber, "response_snippet", string(respBytes[:min(100, len(respBytes))]))
		return nil, fmt.Errorf("failed to decode API response for MC %s: %w", mcNumber, err)
	}

	slog.Info("Successfully fetched and decoded carrier risk profile", "mc_number", mcNumber, "company_name", profile.CompanyName)
	return &profile, nil
}

// doRequestWithAuth handles making HTTP requests to the MCP API, including authorization,
// token refresh, and retries.
func (s *MCPService) doRequestWithAuth(method, urlStr string, bodyData []byte) ([]byte, error) {
	var resp *http.Response
	var err error
	var respBodyBytes []byte

	// Loop for retries (initial attempt + maxRetries)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var req *http.Request // Declare req outside
		req, err = http.NewRequest(method, urlStr, bytes.NewBuffer(bodyData)) // Assign to the existing err
		if err != nil {
			slog.Error("Failed to create HTTP request object", "error", err, "method", method, "url", urlStr)
			return nil, fmt.Errorf("failed to create request for %s: %w", urlStr, err)
		}

		req.Header.Set("Authorization", "Bearer "+s.bearerToken)
		req.Header.Set("Accept", "application/json")
		if method == http.MethodPost || method == http.MethodPut { // Only set Content-Type if there's a body typically
			req.Header.Set("Content-Type", "application/json") // Assuming JSON body for POST/PUT
		}

		slog.Debug("Sending HTTP request to MCP API", "method", method, "url", urlStr, "attempt", attempt)
		resp, err = s.httpClient.Do(req)

		if err != nil {
			slog.Error("HTTP request to MCP API failed", "error", err, "url", urlStr, "attempt", attempt, "method", method)
			if attempt == maxRetries {
				return nil, fmt.Errorf("request to %s failed after %d attempts: %w", urlStr, maxRetries+1, err)
			}
			time.Sleep(time.Duration(attempt+1) * time.Second) // Simple exponential backoff
			continue                                           // Next retry iteration
		}

		// Read body immediately for logging and reuse, then close.
		respBodyBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close() // Close body right after reading
		if err != nil {
			slog.Error("Failed to read response body from MCP API", "error", err, "url", urlStr, "status_code", resp.StatusCode)
			return nil, fmt.Errorf("failed to read response body from %s: %w", urlStr, err)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			slog.Warn("MCP API request returned 401 Unauthorized", "url", urlStr, "attempt", attempt)
			if attempt == maxRetries {
				statusText := fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
				slog.Error("MCP API request unauthorized on final attempt, giving up.", "url", urlStr)
				return nil, fmt.Errorf("unauthorized access to %s after %d attempts (final status: %s)", urlStr, maxRetries+1, statusText)
			}

			errRefresh := s.refreshAuthToken()
			if errRefresh != nil {
				slog.Error("Failed to refresh MCP auth token during request", "error", errRefresh, "url", urlStr)
				return nil, fmt.Errorf("failed to refresh auth token for %s: %w (original status: %s)", urlStr, errRefresh, resp.Status)
			}
			slog.Info("MCP auth token refreshed successfully, retrying original request", "url", urlStr)
			// Loop continues for the retry
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			statusText := fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
			slog.Error("MCP API returned non-success status code", "status_code", resp.StatusCode, "status_text", statusText, "url", urlStr, "method", method, "response_body", string(respBodyBytes))
			return nil, fmt.Errorf("API request to %s failed with status %s: %s", urlStr, statusText, string(respBodyBytes))
		}

		// Successful request
		slog.Debug("MCP API request successful", "method", method, "url", urlStr, "status_code", resp.StatusCode)
		return respBodyBytes, nil
	}

	// This part should ideally not be reached if retry logic is correct and always returns or continues.
	// If it is reached, it means all retries failed without a definitive success or specific error return.
	errMsg := fmt.Sprintf("request to %s failed after %d attempts; last status: unknown (loop exhausted)", urlStr, maxRetries+1)
	if resp != nil { // Should always be non-nil if loop ran at least once without network error
		errMsg = fmt.Sprintf("request to %s failed after %d attempts; last status: %s", urlStr, maxRetries+1, resp.Status)
	}
	slog.Error("MCP request exhausted retries without success", "url", urlStr, "method", method)
	return nil, fmt.Errorf(errMsg)
}

// refreshAuthToken handles the OAuth token refresh logic for the MCP API.
func (s *MCPService) refreshAuthToken() error {
	s.refreshTokenLock.Lock()
	defer s.refreshTokenLock.Unlock()

	slog.Info("Attempting to refresh MCP API token", "endpoint", s.config.TokenEndpointURLMCP)

	if s.config.TokenEndpointURLMCP == "" {
		return fmt.Errorf("MCP token endpoint URL is not configured")
	}
	if s.config.RefreshTokenMCP == "" {
		return fmt.Errorf("MCP refresh token is not configured")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", s.config.RefreshTokenMCP)

	// Use the service's httpClient for the token refresh request to allow mocking.
	req, err := http.NewRequest(http.MethodPost, s.config.TokenEndpointURLMCP, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// Ensure "Authorization" header from the main client is not sent with token requests
	// if it's a default header on s.httpClient. However, s.httpClient.Do doesn't add it by default.
	// Headers on the request 'req' take precedence.

	resp, err := s.httpClient.Do(req) // Use s.httpClient
	if err != nil {
		slog.Error("MCP token refresh HTTP request failed", "error", err, "endpoint", s.config.TokenEndpointURLMCP)
		return fmt.Errorf("token refresh request to %s failed: %w", s.config.TokenEndpointURLMCP, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read token refresh response body", "error", err, "endpoint", s.config.TokenEndpointURLMCP)
		return fmt.Errorf("failed to read token refresh response body from %s: %w", s.config.TokenEndpointURLMCP, err)
	}

	if resp.StatusCode != http.StatusOK {
		statusText := fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		slog.Error("MCP token refresh request returned non-OK status", "status_code", resp.StatusCode, "status_text", statusText, "endpoint", s.config.TokenEndpointURLMCP, "response_body", string(bodyBytes))
		return fmt.Errorf("token refresh for %s failed with status %s: %s", s.config.TokenEndpointURLMCP, statusText, string(bodyBytes))
	}

	var tokenResp models.TokenRefreshResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		slog.Error("Failed to unmarshal token refresh response JSON", "error", err, "endpoint", s.config.TokenEndpointURLMCP, "response_body", string(bodyBytes))
		return fmt.Errorf("failed to decode token refresh response from %s: %w", s.config.TokenEndpointURLMCP, err)
	}

	if tokenResp.AccessToken == "" {
		slog.Error("New access token is empty in MCP token refresh response", "endpoint", s.config.TokenEndpointURLMCP, "response_body", string(bodyBytes))
		return fmt.Errorf("new access token is empty from %s", s.config.TokenEndpointURLMCP)
	}

	s.bearerToken = tokenResp.AccessToken
	slog.Info("MCP API token refreshed successfully. New token obtained.", "endpoint", s.config.TokenEndpointURLMCP)

	// Handle refresh token rotation if the API supports it
	if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != s.config.RefreshTokenMCP {
		slog.Info("MCP API refresh token was also updated by the server. Storing the new refresh token in config for future use.", "endpoint", s.config.TokenEndpointURLMCP)
		// IMPORTANT: This mutates the config object. This change will only persist for the lifetime of this application instance
		// unless the config object is saved back to a persistent store or environment, which is not typical for this kind of app.
		// For a stateless app, the original RefreshTokenMCP from .env should be long-lived, or a more robust system for
		// updating persistent config (like a Vault or encrypted store) would be needed.
		// For now, we update it in the live config struct.
		s.config.RefreshTokenMCP = tokenResp.RefreshToken
	}

	return nil
}

// Helper function min for body snippet logging (already defined in handlers/chat.go, but good to have locally if needed or make it a common util)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
