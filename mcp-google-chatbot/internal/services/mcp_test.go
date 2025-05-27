package services_test

import (
	"bytes"
	"encoding/json"
	// "fmt" // Removed
	"io"
	"net/http"
	// "net/url" // Removed
	// "regexp" // Removed
	"strings"
	"testing"
	// "time" // Removed

	"mcp-google-chatbot/internal/config"
	"mcp-google-chatbot/internal/models"
	"mcp-google-chatbot/internal/services" // Package under test
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRoundTripper is a mock for http.RoundTripper.
type MockRoundTripper struct {
	mock.Mock
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// newMockHttpResponse creates a new http.Response for mocking.
func newMockHttpResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// setupMCPServiceTest creates an MCPService with a mocked HTTP transport and a sample config.
func setupMCPServiceTest(t *testing.T) (*services.MCPService, *MockRoundTripper, *config.Config) {
	mockRT := new(MockRoundTripper)
	testClient := &http.Client{Transport: mockRT}

	cfg := &config.Config{
		BearerTokenMCP:      "initial-bearer-token",
		RefreshTokenMCP:     "initial-refresh-token",
		TokenEndpointURLMCP: "https://mcp.example.com/oauth/token",
		// Add other necessary config fields if MCPService uses them directly
	}

	// Temporarily modify the NewMCPService to accept an *http.Client
	// This is a common way to inject dependencies for testing if the original struct
	// doesn't take an interface. Alternatively, refactor NewMCPService.
	// For this test, we assume we can control the client used by MCPService.
	// The actual NewMCPService creates its own client. So, we'll create the service
	// and then replace its internal client. This is less ideal than constructor injection.

	// To properly test, services.NewMCPService should ideally allow client injection.
	// Let's assume services.MCPService has an exported or settable httpClient field,
	// or NewMCPService is modified for testing.
	// If not, we'd have to use a more complex approach or modify the source.
	// For this exercise, we'll assume we can set the client after creation or
	// that NewMCPService is refactored to accept a client.
	// The provided MCPService implementation in previous steps initializes its own http.Client.
	// We will directly instantiate MCPService for testing to inject the mock client.

	mcpService := services.NewMCPServiceWithClient(cfg, testClient)

	return mcpService, mockRT, cfg
}

const (
	validMCNumber   = "123456"
	mcpAPIBaseURL   = "https://api.mycarrierpackets.com/v1" // Must match the one in mcp.go
	profileEndpoint = "/carrier/" + validMCNumber + "/risk-profile"
)

func TestGetCarrierRiskProfile_Success(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	expectedProfile := models.CarrierRiskProfile{
		MCNumber:    validMCNumber,
		CompanyName: "Test Carrier Inc.",
		OverallRiskScore: 75.5,
		RiskSummary: "Medium",
	}
	expectedBody, _ := json.Marshal(expectedProfile)

	// Expect call to the risk profile endpoint
	expectedURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == expectedURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer initial-bearer-token"
	})).Return(newMockHttpResponse(http.StatusOK, string(expectedBody)), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, expectedProfile.CompanyName, profile.CompanyName)
	assert.Equal(t, expectedProfile.OverallRiskScore, profile.OverallRiskScore)
}

func TestGetCarrierRiskProfile_InvalidMCNumber(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	// No HTTP call expected, so no mockRT.On(...)
	t.Cleanup(func() { mockRT.AssertExpectations(t) }) // Ensures no unexpected calls

	profile, err := mcpService.GetCarrierRiskProfile("INVALID")

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "invalid MC number format")
}

func TestGetCarrierRiskProfile_APIError(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	expectedURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == expectedURL // Compare full URL string
	})).Return(newMockHttpResponse(http.StatusInternalServerError, `{"error":"internal server error"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "API request to") // Check for part of the error message from mcp.go
	assert.Contains(t, err.Error(), "500 Internal Server Error")
}

func TestGetCarrierRiskProfile_TokenRefreshSuccess(t *testing.T) {
	mcpService, mockRT, cfg := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	newAccessToken := "new-access-token"
	tokenRefreshResponse := models.TokenRefreshResponse{
		AccessToken: newAccessToken,
		ExpiresIn:   3600,
	}
	tokenRefreshBody, _ := json.Marshal(tokenRefreshResponse)

	expectedProfile := models.CarrierRiskProfile{MCNumber: validMCNumber, CompanyName: "Refreshed Carrier"}
	profileBody, _ := json.Marshal(expectedProfile)

	// 1. Initial call to GetCarrierRiskProfile - fails with 401
	expectedProfileURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == expectedProfileURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer initial-bearer-token"
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// 2. Call to refresh token endpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		if req.Method != http.MethodPost || req.URL.String() != cfg.TokenEndpointURLMCP {
			return false
		}
		bodyBytes, _ := io.ReadAll(req.Body) // Read the body to check its content
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Important: Restore the body
		bodyStr := string(bodyBytes)
		return strings.Contains(bodyStr, "grant_type=refresh_token") &&
			strings.Contains(bodyStr, "refresh_token="+cfg.RefreshTokenMCP)
	})).Return(newMockHttpResponse(http.StatusOK, string(tokenRefreshBody)), nil).Once()

	// 3. Retry call to GetCarrierRiskProfile - succeeds with new token
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet &&
			req.URL.String() == expectedProfileURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer "+newAccessToken // Check for new token
	})).Return(newMockHttpResponse(http.StatusOK, string(profileBody)), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "Refreshed Carrier", profile.CompanyName)

	// Verify that the service's internal token was updated (if possible to inspect,
	// otherwise the third mock call's Authorization header check implicitly does this)
	// For this, we'd need a way to get the current token from mcpService or
	// mcpService.bearerToken needs to be exported for test inspection (not ideal).
	// The current structure of MCPService does not export bearerToken.
	// The check in the third mockRT.On(...) call is the practical way to verify this.
}

func TestGetCarrierRiskProfile_TokenRefreshFails(t *testing.T) {
	mcpService, mockRT, cfg := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	// 1. Initial call - 401
	expectedProfileURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == expectedProfileURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer initial-bearer-token"
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// 2. Refresh token call - fails
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == cfg.TokenEndpointURLMCP
	})).Return(newMockHttpResponse(http.StatusInternalServerError, `{"error":"refresh failed"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "failed to refresh auth token") // Check for part of the error from mcp.go
	assert.Contains(t, err.Error(), "token refresh for")
	assert.Contains(t, err.Error(), "failed with status 500 Internal Server Error")
}

func TestGetCarrierRiskProfile_TokenRefreshSuccessRetryFails(t *testing.T) {
	mcpService, mockRT, cfg := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	newAccessToken := "new-access-token-for-retry-fail"
	tokenRefreshResponse := models.TokenRefreshResponse{AccessToken: newAccessToken, ExpiresIn: 3600}
	tokenRefreshBody, _ := json.Marshal(tokenRefreshResponse)

	// 1. Initial call - 401
	expectedProfileURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == expectedProfileURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer initial-bearer-token"
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// 2. Refresh token call - success
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.String() == cfg.TokenEndpointURLMCP
	})).Return(newMockHttpResponse(http.StatusOK, string(tokenRefreshBody)), nil).Once()

	// 3. Retry call - fails again with 401 (e.g., new token is also invalid immediately)
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == expectedProfileURL && // Compare full URL string
			req.Header.Get("Authorization") == "Bearer "+newAccessToken
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"new token also invalid"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	// This error comes from doRequestWithAuth after a successful refresh but failed retry
	assert.Contains(t, err.Error(), "unauthorized access to")
	// maxRetries is 1, so total attempts = maxRetries + 1 = 2
	assert.Contains(t, err.Error(), "after 2 attempts (final status: 401 Unauthorized)")
}

// TestGetCarrierRiskProfile_InvalidJSONResponse tests handling of malformed JSON from the API.
func TestGetCarrierRiskProfile_InvalidJSONResponse(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	expectedURL := mcpAPIBaseURL + profileEndpoint
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == expectedURL // Compare full URL string
	})).Return(newMockHttpResponse(http.StatusOK, `{"mc_number":123, "company_name":"Test"}`), nil).Once() // mc_number is int, not string

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "failed to decode API response")
}
