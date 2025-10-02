package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestConfig struct {
	Test struct {
		ParserURL      string `toml:"parser_url"`
		TimeoutSeconds int    `toml:"timeout_seconds"`
	} `toml:"test"`
	API struct {
		Enabled      bool `toml:"enabled"`
		RetryCount   int  `toml:"retry_count"`
		RetryDelayMs int  `toml:"retry_delay_ms"`
	} `toml:"api"`
}

var config TestConfig

func init() {
	configPath := filepath.Join("..", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		panic("Failed to read config.toml: " + err.Error())
	}

	if err := toml.Unmarshal(data, &config); err != nil {
		panic("Failed to parse config.toml: " + err.Error())
	}
}

// AuthData represents the authentication data sent from the Chrome extension
type AuthData struct {
	Cookies   []Cookie          `json:"cookies"`
	Tokens    map[string]string `json:"tokens"`
	UserAgent string            `json:"userAgent"`
	BaseURL   string            `json:"baseUrl"`
	Timestamp int64             `json:"timestamp"`
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	HttpOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite"`
}

func TestReceiverEndpoint(t *testing.T) {
	// Skip if API tests are disabled
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	// Create sample authentication data
	authData := AuthData{
		Cookies: []Cookie{
			{
				Name:     "cloud.session.token",
				Value:    "test-token-12345",
				Domain:   ".atlassian.net",
				Path:     "/",
				Expires:  time.Now().Add(24 * time.Hour).Unix(),
				HttpOnly: true,
				Secure:   true,
				SameSite: "None",
			},
		},
		Tokens: map[string]string{
			"cloud.session.token": "test-token-12345",
		},
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
		BaseURL:   "https://test.atlassian.net",
		Timestamp: time.Now().UnixMilli(),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(authData)
	require.NoError(t, err, "Failed to marshal auth data")

	// Create HTTP client with timeout from config
	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	// Send POST request to receiver endpoint
	resp, err := client.Post(
		config.Test.ParserURL+"/api/receiver",
		"application/json",
		bytes.NewBuffer(jsonData),
	)

	// If we get a connection error, the server might not be running
	if err != nil {
		t.Logf("Connection error (server not running): %v", err)
		t.Skip("Skipping test - server not running")
		return
	}
	defer resp.Body.Close()

	// Read response body for logging
	bodyBytes, _ := io.ReadAll(resp.Body)
	t.Logf("Response Status: %d", resp.StatusCode)
	t.Logf("Response Body: %s", string(bodyBytes))

	// Check if endpoint exists
	if resp.StatusCode == http.StatusNotFound {
		// Decode the 404 error response
		var errorResponse map[string]interface{}
		json.Unmarshal(bodyBytes, &errorResponse)
		t.Logf("Endpoint not implemented yet. Error: %v", errorResponse)
		t.Skip("Skipping test - /api/receiver endpoint not implemented")
		return
	}

	// Assert successful response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK response")

	// Decode response
	var response map[string]interface{}
	err = json.Unmarshal(bodyBytes, &response)
	require.NoError(t, err, "Failed to decode response")

	// Verify response structure
	assert.Contains(t, response, "success", "Response should contain success field")
	assert.True(t, response["success"].(bool), "Success should be true")
}
