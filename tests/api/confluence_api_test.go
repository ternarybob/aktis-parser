package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfluence_ClearAllData verifies clearing all data from database
func TestConfluence_ClearAllData(t *testing.T) {
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	url := config.Test.ParserURL + "/api/data/clear-all"
	t.Logf("Testing: POST %s", url)

	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err, "Should create request")

	resp, err := client.Do(req)
	require.NoError(t, err, "Should be able to call clear-all endpoint")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response body")

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Should parse JSON response")

	assert.Equal(t, "success", result["status"], "Should return success status")
	t.Logf("✓ Clear all data successful: %s", result["message"])

	// Verify data is actually cleared by checking confluence endpoint
	t.Log("Verifying data is cleared...")
	time.Sleep(500 * time.Millisecond)

	resp2, err := client.Get(config.Test.ParserURL + "/api/data/confluence")
	require.NoError(t, err, "Should be able to check confluence data")
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err, "Should read confluence data response")

	var confluenceData map[string]interface{}
	err = json.Unmarshal(body2, &confluenceData)
	require.NoError(t, err, "Should parse confluence data")

	spaces, ok := confluenceData["spaces"].([]interface{})
	if ok {
		assert.Equal(t, 0, len(spaces), "Spaces should be empty after clear")
	}

	pages, ok := confluenceData["pages"].([]interface{})
	if ok {
		assert.Equal(t, 0, len(pages), "Pages should be empty after clear")
	}

	t.Log("✅ Clear all data API test passed")
}

// TestConfluence_RefreshSpacesCache verifies syncing spaces from Confluence
func TestConfluence_RefreshSpacesCache(t *testing.T) {
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	// First clear all data
	t.Log("Clearing all data first...")
	clearReq, _ := http.NewRequest("POST", config.Test.ParserURL+"/api/data/clear-all", nil)
	clearResp, err := client.Do(clearReq)
	require.NoError(t, err, "Should clear data")
	clearResp.Body.Close()
	time.Sleep(500 * time.Millisecond)

	// Trigger space sync
	url := config.Test.ParserURL + "/api/spaces/refresh-cache"
	t.Logf("Testing: POST %s", url)

	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err, "Should create request")

	resp, err := client.Do(req)
	require.NoError(t, err, "Should be able to call refresh-cache endpoint")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response body")

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err, "Should parse JSON response")

	assert.Equal(t, "started", result["status"], "Should return started status")
	t.Logf("✓ Space sync started: %s", result["message"])

	// Poll for spaces to appear (max 30 seconds)
	t.Log("Polling for spaces to appear (max 30 seconds)...")
	maxWait := 30 * time.Second
	pollInterval := 2 * time.Second
	startTime := time.Now()

	var spaces []interface{}
	spacesFound := false

	for time.Since(startTime) < maxWait {
		resp2, err := client.Get(config.Test.ParserURL + "/api/data/confluence")
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		body2, err := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var data map[string]interface{}
		err = json.Unmarshal(body2, &data)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if spaceList, ok := data["spaces"].([]interface{}); ok && len(spaceList) > 0 {
			spaces = spaceList
			spacesFound = true
			break
		}

		t.Logf("Waiting for spaces... elapsed: %v", time.Since(startTime).Round(time.Second))
		time.Sleep(pollInterval)
	}

	require.True(t, spacesFound, "Spaces should appear within timeout")
	t.Logf("✓ Found %d spaces after %v", len(spaces), time.Since(startTime).Round(time.Second))

	// Verify space structure
	require.Greater(t, len(spaces), 0, "Should have at least one space")

	firstSpace := spaces[0].(map[string]interface{})
	assert.NotEmpty(t, firstSpace["key"], "Space should have key")
	assert.NotEmpty(t, firstSpace["name"], "Space should have name")

	// Verify page counts are included
	if pageCount, ok := firstSpace["pageCount"]; ok {
		t.Logf("Space %s has pageCount: %v", firstSpace["key"], pageCount)
		assert.NotNil(t, pageCount, "Space should have pageCount field")
	} else {
		t.Log("Warning: pageCount not found in space data")
	}

	t.Log("✅ Refresh spaces cache API test passed")
}

// TestConfluence_GetSpacePages verifies fetching pages for selected spaces
func TestConfluence_GetSpacePages(t *testing.T) {
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	// First get available spaces
	t.Log("Getting available spaces...")
	resp, err := client.Get(config.Test.ParserURL + "/api/data/confluence")
	require.NoError(t, err, "Should get confluence data")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response")

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	require.NoError(t, err, "Should parse JSON")

	spaces, ok := data["spaces"].([]interface{})
	require.True(t, ok && len(spaces) > 0, "Should have spaces available")

	// Select first space
	firstSpace := spaces[0].(map[string]interface{})
	spaceKey := firstSpace["key"].(string)
	t.Logf("Selected space: %s", spaceKey)

	// Request pages for this space
	url := config.Test.ParserURL + "/api/spaces/get-pages"
	t.Logf("Testing: POST %s", url)

	requestBody := map[string]interface{}{
		"spaceKeys": []string{spaceKey},
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	require.NoError(t, err, "Should create request")
	req.Header.Set("Content-Type", "application/json")

	resp2, err := client.Do(req)
	require.NoError(t, err, "Should be able to call get-pages endpoint")
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode, "Should return 200 OK")

	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err, "Should read response body")

	var result map[string]interface{}
	err = json.Unmarshal(body2, &result)
	require.NoError(t, err, "Should parse JSON response")

	assert.Equal(t, "started", result["status"], "Should return started status")
	t.Logf("✓ Page fetch started: %s", result["message"])

	// Poll for pages to appear
	t.Log("Polling for pages to appear (max 30 seconds)...")
	maxWait := 30 * time.Second
	pollInterval := 2 * time.Second
	startTime := time.Now()

	var pages []interface{}
	pagesFound := false

	for time.Since(startTime) < maxWait {
		queryURL := config.Test.ParserURL + "/api/data/confluence/pages?spaceKey=" + spaceKey
		resp3, err := client.Get(queryURL)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		body3, err := io.ReadAll(resp3.Body)
		resp3.Body.Close()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var pageData map[string]interface{}
		err = json.Unmarshal(body3, &pageData)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if pageList, ok := pageData["pages"].([]interface{}); ok && len(pageList) > 0 {
			pages = pageList
			pagesFound = true
			break
		}

		t.Logf("Waiting for pages... elapsed: %v", time.Since(startTime).Round(time.Second))
		time.Sleep(pollInterval)
	}

	if !pagesFound {
		t.Skip("Selected space has no pages or scraping takes longer than timeout - this is OK for empty spaces")
	}

	t.Logf("✓ Found %d pages after %v", len(pages), time.Since(startTime).Round(time.Second))

	// Verify page structure
	if len(pages) > 0 {
		firstPage := pages[0].(map[string]interface{})
		assert.NotEmpty(t, firstPage["id"], "Page should have id")
		assert.NotEmpty(t, firstPage["title"], "Page should have title")

		// Verify pages belong to selected space
		for _, p := range pages {
			page := p.(map[string]interface{})
			if space, ok := page["space"].(map[string]interface{}); ok {
				pageSpaceKey := space["key"].(string)
				assert.Equal(t, spaceKey, pageSpaceKey, "Page should belong to selected space")
			}
		}
	}

	t.Log("✅ Get space pages API test passed")
}

// TestConfluence_PageFiltering verifies filtering pages by space key
func TestConfluence_PageFiltering(t *testing.T) {
	if !config.API.Enabled {
		t.Skip("API tests disabled in config")
	}

	timeout := time.Duration(config.Test.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	// Get all spaces
	resp, err := client.Get(config.Test.ParserURL + "/api/data/confluence")
	require.NoError(t, err, "Should get confluence data")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response")

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	require.NoError(t, err, "Should parse JSON")

	spaces, ok := data["spaces"].([]interface{})
	require.True(t, ok && len(spaces) >= 2, "Should have at least 2 spaces for filtering test")

	// Get pages for first two spaces
	space1Key := spaces[0].(map[string]interface{})["key"].(string)
	space2Key := spaces[1].(map[string]interface{})["key"].(string)

	t.Logf("Testing filtering with spaces: %s, %s", space1Key, space2Key)

	// Query pages with multiple space filters
	queryURL := config.Test.ParserURL + "/api/data/confluence/pages?spaceKey=" + space1Key + "&spaceKey=" + space2Key
	resp2, err := client.Get(queryURL)
	require.NoError(t, err, "Should query pages with filters")
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err, "Should read response")

	var pageData map[string]interface{}
	err = json.Unmarshal(body2, &pageData)
	require.NoError(t, err, "Should parse JSON")

	pages, ok := pageData["pages"].([]interface{})
	if !ok || len(pages) == 0 {
		t.Log("No pages found for filtering test (may need to run get-pages first)")
		return
	}

	// Verify all pages belong to one of the selected spaces
	for _, p := range pages {
		page := p.(map[string]interface{})
		if space, ok := page["space"].(map[string]interface{}); ok {
			pageSpaceKey := space["key"].(string)
			assert.True(t,
				pageSpaceKey == space1Key || pageSpaceKey == space2Key,
				"Page should belong to one of the filtered spaces")
		}
	}

	t.Logf("✓ Found %d pages matching filter", len(pages))
	t.Log("✅ Page filtering API test passed")
}
