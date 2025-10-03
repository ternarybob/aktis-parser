package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// PaginationResponse matches the response structure from collector endpoints
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// CollectorResponse matches the response structure from collector endpoints
type CollectorResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Pagination PaginationResponse       `json:"pagination"`
}

// TestCollector_GetProjects verifies getting project index with counts
func TestCollector_GetProjects(t *testing.T) {
	t.Log("Getting projects index...")

	url := config.Test.ParserURL + "/api/collector/projects"
	resp, err := http.Get(url)
	require.NoError(t, err, "Should be able to call projects endpoint")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should be able to read response body")

	var response CollectorResponse
	err = json.Unmarshal(body, &response)
	require.NoError(t, err, "Should be able to parse JSON response")

	t.Logf("Projects found: %d", response.Pagination.TotalItems)
	t.Logf("Pagination: page=%d, pageSize=%d, totalPages=%d",
		response.Pagination.Page,
		response.Pagination.PageSize,
		response.Pagination.TotalPages)

	require.GreaterOrEqual(t, response.Pagination.TotalItems, 0, "Should have valid total items")
	require.GreaterOrEqual(t, response.Pagination.TotalPages, 0, "Should have valid total pages")

	// Verify each project has key and issueCount
	for i, project := range response.Data {
		key, hasKey := project["key"].(string)
		require.True(t, hasKey, fmt.Sprintf("Project %d should have key", i))
		require.NotEmpty(t, key, fmt.Sprintf("Project %d key should not be empty", i))

		issueCount, hasCount := project["issueCount"].(float64)
		require.True(t, hasCount, fmt.Sprintf("Project %s should have issueCount", key))
		require.GreaterOrEqual(t, int(issueCount), 0, fmt.Sprintf("Project %s should have valid issueCount", key))

		t.Logf("  Project: %s, Issues: %d", key, int(issueCount))
	}

	t.Log("✅ Projects endpoint verified successfully")
}

// TestCollector_GetIssuesPaginated verifies paginated issue retrieval
func TestCollector_GetIssuesPaginated(t *testing.T) {
	// First, get projects to find one with issues
	t.Log("Getting projects to find one with issues...")
	projectsURL := config.Test.ParserURL + "/api/collector/projects"
	resp, err := http.Get(projectsURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var projectsResponse CollectorResponse
	err = json.Unmarshal(body, &projectsResponse)
	require.NoError(t, err)

	// Find a project with issues
	var testProjectKey string
	var expectedIssueCount int
	for _, project := range projectsResponse.Data {
		if count, ok := project["issueCount"].(float64); ok && count > 0 {
			testProjectKey = project["key"].(string)
			expectedIssueCount = int(count)
			break
		}
	}

	if testProjectKey == "" {
		t.Skip("No projects with issues found (run scrape first)")
	}

	t.Logf("Testing pagination with project: %s (expected %d issues)", testProjectKey, expectedIssueCount)

	// Test page 1 with pageSize=10
	pageSize := 10
	page := 0

	t.Logf("Fetching page %d with pageSize=%d...", page, pageSize)
	issuesURL := fmt.Sprintf("%s/api/collector/issues?projectKey=%s&page=%d&pageSize=%d",
		config.Test.ParserURL, testProjectKey, page, pageSize)

	resp, err = http.Get(issuesURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	var issuesResponse CollectorResponse
	err = json.Unmarshal(body, &issuesResponse)
	require.NoError(t, err)

	t.Logf("Page %d results: %d issues", page, len(issuesResponse.Data))
	t.Logf("Pagination: totalItems=%d, totalPages=%d",
		issuesResponse.Pagination.TotalItems,
		issuesResponse.Pagination.TotalPages)

	require.Equal(t, expectedIssueCount, issuesResponse.Pagination.TotalItems,
		"Total items should match project issue count")
	require.Equal(t, page, issuesResponse.Pagination.Page, "Page number should match request")
	require.Equal(t, pageSize, issuesResponse.Pagination.PageSize, "Page size should match request")

	expectedPages := (expectedIssueCount + pageSize - 1) / pageSize
	require.Equal(t, expectedPages, issuesResponse.Pagination.TotalPages,
		"Total pages should be correct")

	// Verify we got the right number of items for this page
	expectedItemsOnPage := pageSize
	if expectedIssueCount < pageSize {
		expectedItemsOnPage = expectedIssueCount
	}
	require.Equal(t, expectedItemsOnPage, len(issuesResponse.Data),
		"Should return correct number of items for page")

	// Verify each issue belongs to the correct project
	for i, issue := range issuesResponse.Data {
		fields, hasFields := issue["fields"].(map[string]interface{})
		require.True(t, hasFields, fmt.Sprintf("Issue %d should have fields", i))

		project, hasProject := fields["project"].(map[string]interface{})
		require.True(t, hasProject, fmt.Sprintf("Issue %d should have project in fields", i))

		projectKey, hasKey := project["key"].(string)
		require.True(t, hasKey, fmt.Sprintf("Issue %d project should have key", i))
		require.Equal(t, testProjectKey, projectKey,
			fmt.Sprintf("Issue %d should belong to project %s", i, testProjectKey))
	}

	// Test fetching all pages
	t.Log("Fetching all pages...")
	allIssues := make([]map[string]interface{}, 0)
	allIssues = append(allIssues, issuesResponse.Data...)

	for p := 1; p < issuesResponse.Pagination.TotalPages; p++ {
		t.Logf("Fetching page %d...", p)
		pageURL := fmt.Sprintf("%s/api/collector/issues?projectKey=%s&page=%d&pageSize=%d",
			config.Test.ParserURL, testProjectKey, p, pageSize)

		pageResp, err := http.Get(pageURL)
		require.NoError(t, err)

		pageBody, err := io.ReadAll(pageResp.Body)
		require.NoError(t, err)
		pageResp.Body.Close()

		var pageIssuesResponse CollectorResponse
		err = json.Unmarshal(pageBody, &pageIssuesResponse)
		require.NoError(t, err)

		t.Logf("  Page %d: %d issues", p, len(pageIssuesResponse.Data))
		allIssues = append(allIssues, pageIssuesResponse.Data...)
	}

	require.Equal(t, expectedIssueCount, len(allIssues),
		"Should have fetched all issues across all pages")

	t.Logf("✅ Successfully paginated through all %d issues in %d pages",
		len(allIssues), issuesResponse.Pagination.TotalPages)
}

// TestCollector_GetSpaces verifies getting Confluence spaces with page counts
func TestCollector_GetSpaces(t *testing.T) {
	t.Log("Getting Confluence spaces...")

	url := config.Test.ParserURL + "/api/collector/spaces"
	resp, err := http.Get(url)
	require.NoError(t, err, "Should be able to call spaces endpoint")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should be able to read response body")

	var response CollectorResponse
	err = json.Unmarshal(body, &response)
	require.NoError(t, err, "Should be able to parse JSON response")

	t.Logf("Spaces found: %d", response.Pagination.TotalItems)

	// Verify each space has key and pageCount
	for i, space := range response.Data {
		key, hasKey := space["key"].(string)
		require.True(t, hasKey, fmt.Sprintf("Space %d should have key", i))
		require.NotEmpty(t, key, fmt.Sprintf("Space %d key should not be empty", i))

		if pageCount, hasCount := space["pageCount"].(float64); hasCount {
			t.Logf("  Space: %s, Pages: %d", key, int(pageCount))
		} else {
			t.Logf("  Space: %s, Pages: not available", key)
		}
	}

	t.Log("✅ Spaces endpoint verified successfully")
}

// TestCollector_GetPagesPaginated verifies paginated page retrieval for a space
func TestCollector_GetPagesPaginated(t *testing.T) {
	// First, get spaces to find one with pages
	t.Log("Getting spaces to find one with pages...")
	spacesURL := config.Test.ParserURL + "/api/collector/spaces"
	resp, err := http.Get(spacesURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spacesResponse CollectorResponse
	err = json.Unmarshal(body, &spacesResponse)
	require.NoError(t, err)

	// Find a space with pages
	var testSpaceKey string
	for _, space := range spacesResponse.Data {
		if count, ok := space["pageCount"].(float64); ok && count > 0 {
			testSpaceKey = space["key"].(string)
			break
		}
	}

	if testSpaceKey == "" {
		t.Skip("No spaces with pages found (run GET PAGES first)")
	}

	t.Logf("Testing pagination with space: %s", testSpaceKey)

	// Test page 1 with pageSize=10
	pageSize := 10
	page := 0

	pagesURL := fmt.Sprintf("%s/api/collector/pages?spaceKey=%s&page=%d&pageSize=%d",
		config.Test.ParserURL, testSpaceKey, page, pageSize)

	resp, err = http.Get(pagesURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	var pagesResponse CollectorResponse
	err = json.Unmarshal(body, &pagesResponse)
	require.NoError(t, err)

	t.Logf("Page %d results: %d pages", page, len(pagesResponse.Data))
	t.Logf("Pagination: totalItems=%d, totalPages=%d",
		pagesResponse.Pagination.TotalItems,
		pagesResponse.Pagination.TotalPages)

	// Verify each page belongs to the correct space
	for i, page := range pagesResponse.Data {
		space, hasSpace := page["space"].(map[string]interface{})
		require.True(t, hasSpace, fmt.Sprintf("Page %d should have space", i))

		spaceKey, hasKey := space["key"].(string)
		require.True(t, hasKey, fmt.Sprintf("Page %d space should have key", i))
		require.Equal(t, testSpaceKey, spaceKey,
			fmt.Sprintf("Page %d should belong to space %s", i, testSpaceKey))
	}

	t.Logf("✅ Pages endpoint verified successfully with %d pages", len(pagesResponse.Data))
}
