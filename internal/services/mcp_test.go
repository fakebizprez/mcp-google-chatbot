package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mcp-google-chatbot/internal/config"
	"mcp-google-chatbot/internal/models"
	// "mcp-google-chatbot/internal/services"; // Already in 'services' package
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRoundTripper is a mock implementation of http.RoundTripper for testing.
type MockRoundTripper struct {
	mock.Mock
}

// RoundTrip implements the http.RoundTripper interface.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// newMockHttpResponse creates a new mock HTTP response.
func newMockHttpResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// setupMCPServiceTest is a helper function to set up MCPService with a mock HTTP client for tests.
func setupMCPServiceTest(t *testing.T) (*MCPService, *MockRoundTripper, *config.Config) {
	t.Helper()
	mockRT := new(MockRoundTripper)
	testClient := &http.Client{
		Transport: mockRT,
		Timeout:   time.Second * 5, // Reasonable timeout for tests
	}

	// Minimal config for testing
	cfg := &config.Config{
		BearerTokenMCP:      "initial-bearer-token",
		RefreshTokenMCP:     "test-refresh-token",
		TokenEndpointURLMCP: "https://mockauth.example.com/token", // Mock token endpoint
		ClientIDMCP:         "test-client-id",
		ClientSecretMCP:     "test-client-secret",
	}

	mcpService := NewMCPServiceWithClient(cfg, testClient)
	return mcpService, mockRT, cfg
}

const (
	validMCNumber   = "123456"
	invalidMCNumber = "ABC" // Example of an invalid format
)

// TestGetCarrierRiskProfile_Success tests successful retrieval and parsing of a carrier profile.
func TestGetCarrierRiskProfile_Success(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	expectedCarrierData := models.MCPCarrierData{
		DocketNumber: validMCNumber, // MC Number
		CompanyName:  "Test Carrier Inc.",
		RiskAssessmentDetails: &models.RiskAssessmentDetails{
			TotalPoints: 75.5,
			// Other sub-fields can be added if desired for more thorough testing
		},
		// Add other fields from MCPCarrierData as needed for assertion
	}
	expectedResponse := models.MCPPreviewResponse{expectedCarrierData} // API returns an array
	expectedBodyBytes, _ := json.Marshal(expectedResponse)
	expectedBody := string(expectedBodyBytes)

	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && // Changed to POST
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber && // Check query param
			req.Header.Get("Authorization") == "Bearer initial-bearer-token" &&
			req.Body == nil // Body should be nil for POST with query params
	})).Return(newMockHttpResponse(http.StatusOK, expectedBody), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, expectedCarrierData.CompanyName, profile.CompanyName)
	if assert.NotNil(t, profile.RiskAssessmentDetails) { // Check for nil before accessing
		assert.Equal(t, expectedCarrierData.RiskAssessmentDetails.TotalPoints, profile.RiskAssessmentDetails.TotalPoints)
	} else {
		t.Errorf("Expected RiskAssessmentDetails to be populated but it was nil") // More direct error
	}
}

// TestGetCarrierRiskProfile_NoDataFound tests the case where the API returns an empty array (no data).
func TestGetCarrierRiskProfile_NoDataFound(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	emptyResponse := models.MCPPreviewResponse{} // Empty array
	emptyBodyBytes, _ := json.Marshal(emptyResponse)
	emptyBody := string(emptyBodyBytes)
	
	mcNumberForTest := "654321" // Use a different MC for this specific test

	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == mcNumberForTest &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusOK, emptyBody), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(mcNumberForTest)

	assert.Error(t, err)
	assert.Nil(t, profile)
	if err != nil { // Defensive check, assert.Error should ensure this
		assert.Contains(t, err.Error(), fmt.Sprintf("no data found for MC number %s", mcNumberForTest))
	}
}

// TestGetCarrierRiskProfile_InvalidFormat tests the handling of an invalid MC number format.
func TestGetCarrierRiskProfile_InvalidFormat(t *testing.T) {
	mcpService, _, _ := setupMCPServiceTest(t) // MockRoundTripper not strictly needed as it shouldn't be called

	profile, err := mcpService.GetCarrierRiskProfile(invalidMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	if err != nil {
		assert.Contains(t, err.Error(), "invalid MC number format")
	}
}

// TestGetCarrierRiskProfile_APIError tests handling of a generic API error (e.g., 500).
func TestGetCarrierRiskProfile_APIError(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusInternalServerError, `{"error":"internal server error"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	if err != nil {
		// The error from doRequestWithAuth for a non-2xx status includes the status code and body
		expectedURL := fmt.Sprintf("https://mycarrierpacketsapi-stage.azurewebsites.net/api/v1/Carrier/PreviewCarrier?docketNumber=%s", validMCNumber)
		assert.Contains(t, err.Error(), fmt.Sprintf("API request to %s failed with status 500 Internal Server Error: {\"error\":\"internal server error\"}", expectedURL))
	}
}

// TestGetCarrierRiskProfile_TokenRefreshSuccess tests token refresh and subsequent successful call.
func TestGetCarrierRiskProfile_TokenRefreshSuccess(t *testing.T) {
	mcpService, mockRT, cfg := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	// Expected data for the successful call after refresh
	expectedProfileData := models.MCPCarrierData{DocketNumber: validMCNumber, CompanyName: "Refreshed Carrier"}
	expectedResponse := models.MCPPreviewResponse{expectedProfileData}
	profileBodyBytes, _ := json.Marshal(expectedResponse)
	// profileBodyString := string(profileBodyBytes) // Used in the mock return

	// Mock the token refresh response
	newAccessToken := "refreshed-bearer-token"
	tokenRefreshResponse := models.TokenRefreshResponse{AccessToken: newAccessToken, ExpiresIn: 3600}
	tokenRefreshBodyBytes, _ := json.Marshal(tokenRefreshResponse)
	tokenRefreshBody := string(tokenRefreshBodyBytes)

	// First call: Unauthorized
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Header.Get("Authorization") == "Bearer initial-bearer-token" &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// Token refresh call - this part remains the same
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.Host == "mockauth.example.com" && req.URL.Path == "/token"
	})).Return(newMockHttpResponse(http.StatusOK, tokenRefreshBody), nil).Once()

	// Second call (Retry): Success with new token
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Header.Get("Authorization") == "Bearer "+newAccessToken && // Check for new token
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusOK, string(profileBodyBytes)), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "Refreshed Carrier", profile.CompanyName)
	assert.Equal(t, newAccessToken, cfg.BearerTokenMCP, "Bearer token in config should be updated after refresh")
}

// TestGetCarrierRiskProfile_HTTPError tests generic HTTP error during the request.
func TestGetCarrierRiskProfile_HTTPError(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	// For HTTP errors, the URL matching might be less specific if the error occurs before full URL parsing,
	// but for consistency with other tests, we keep the specific match.
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Body == nil
	})).Return(nil, fmt.Errorf("simulated net error")).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	if err != nil {
		assert.Contains(t, err.Error(), "simulated net error")
	}
}

// TestGetCarrierRiskProfile_UnmarshalError tests malformed JSON response from API.
// This was the original name, let's keep it for now.
func TestGetCarrierRiskProfile_UnmarshalError(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	malformedJSON := `{"CompanyName":"Test", "DotNumber":123` // Missing closing brace and quote

	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusOK, malformedJSON), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "failed to decode API response") || strings.Contains(err.Error(), "unexpected end of JSON input"), "Error message should indicate JSON decoding failure.")
	}
}

// TestGetCarrierRiskProfile_TokenRefreshFails tests when token refresh itself fails.
func TestGetCarrierRiskProfile_TokenRefreshFails(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	// First call: Unauthorized
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Header.Get("Authorization") == "Bearer initial-bearer-token" &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// Token refresh call fails
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.Host == "mockauth.example.com" && req.URL.Path == "/token"
	})).Return(newMockHttpResponse(http.StatusInternalServerError, `{"error":"refresh failed"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "failed to refresh auth token")
	assert.Contains(t, err.Error(), "refresh failed") // Check that the error from refresh is propagated
}

// TestGetCarrierRiskProfile_TokenRefreshSuccessRetryFails tests when token refresh succeeds but the retry still fails.
func TestGetCarrierRiskProfile_TokenRefreshSuccessRetryFails(t *testing.T) {
	mcpService, mockRT, cfg := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	newAccessToken := "refreshed-bearer-token"
	tokenRefreshResponse := models.TokenRefreshResponse{AccessToken: newAccessToken, ExpiresIn: 3600}
	tokenRefreshBodyBytes, _ := json.Marshal(tokenRefreshResponse)
	tokenRefreshBody := string(tokenRefreshBodyBytes)

	// First call: Unauthorized
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Header.Get("Authorization") == "Bearer initial-bearer-token" &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusUnauthorized, `{"error":"token expired"}`), nil).Once()

	// Token refresh call (succeeds)
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.Host == "mockauth.example.com" && req.URL.Path == "/token"
	})).Return(newMockHttpResponse(http.StatusOK, tokenRefreshBody), nil).Once()

	// Second call (Retry): Fails again (e.g., with 403 Forbidden)
	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Header.Get("Authorization") == "Bearer "+newAccessToken && // Uses new token
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusForbidden, `{"error":"forbidden after refresh"}`), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	// Error message should reflect the failure on the second attempt.
	expectedURL := fmt.Sprintf("https://mycarrierpacketsapi-stage.azurewebsites.net/api/v1/Carrier/PreviewCarrier?docketNumber=%s", validMCNumber)
	assert.Contains(t, err.Error(), fmt.Sprintf("API request to %s failed with status 403 Forbidden: {\"error\":\"forbidden after refresh\"}", expectedURL))
	assert.Equal(t, newAccessToken, cfg.BearerTokenMCP, "Bearer token in config should still be updated after successful refresh")
}

// TestGetCarrierRiskProfile_InvalidJSONResponse tests a response that is valid JSON but not the expected structure.
func TestGetCarrierRiskProfile_InvalidJSONResponse(t *testing.T) {
	mcpService, mockRT, _ := setupMCPServiceTest(t)
	t.Cleanup(func() { mockRT.AssertExpectations(t) })

	// Valid JSON, but not MCPPreviewResponse structure. Example: CompanyName is a number.
	invalidStructJSON := `[{"CompanyName": 123, "DotNumber": "789012"}]`

	mockRT.On("RoundTrip", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost &&
			req.URL.Scheme == "https" &&
			req.URL.Host == "mycarrierpacketsapi-stage.azurewebsites.net" &&
			req.URL.Path == "/api/v1/Carrier/PreviewCarrier" &&
			req.URL.Query().Get("docketNumber") == validMCNumber &&
			req.Body == nil
	})).Return(newMockHttpResponse(http.StatusOK, invalidStructJSON), nil).Once()

	profile, err := mcpService.GetCarrierRiskProfile(validMCNumber)

	assert.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "failed to decode API response")
}
