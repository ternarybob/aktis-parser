package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIAvailability(t *testing.T) {
	// Skip if API tests are disabled
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	// Create HTTP client with timeout from config
	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	// Test root endpoint
	resp, err := client.Get(config.Test.ParserURL + "/")
	require.NoError(t, err, "Should be able to connect to API server")
	defer resp.Body.Close()

	// API is available if we get any response (even 404)
	assert.NotNil(t, resp, "Should receive a response from server")
	t.Logf("API is available at %s (status: %d)", config.Test.ParserURL, resp.StatusCode)
}
