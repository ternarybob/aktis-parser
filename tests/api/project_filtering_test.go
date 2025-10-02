package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const baseURL = "http://localhost:8085"

func TestProjectIssueFiltering(t *testing.T) {
	t.Log("=== Starting Project Issue Filtering Test ===")

	// Step 1: Clear all data
	t.Log("Step 1: Clearing all data...")
	resp, err := http.Post(baseURL+"/api/data/clear-all", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to clear data: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Clear data failed with status: %d", resp.StatusCode)
	}
	t.Log("✓ Data cleared successfully")
	time.Sleep(1 * time.Second)

	// Step 2: Refresh projects cache
	t.Log("Step 2: Refreshing projects cache...")
	resp, err = http.Post(baseURL+"/api/projects/refresh-cache", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Refresh cache failed with status: %d", resp.StatusCode)
	}
	t.Log("✓ Projects cache refreshed")
	time.Sleep(3 * time.Second) // Wait for projects to be fetched

	// Step 3: Get list of projects
	t.Log("Step 3: Getting project list...")
	resp, err = http.Get(baseURL + "/api/data/jira")
	if err != nil {
		t.Fatalf("Failed to get projects: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var projectData struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.Unmarshal(body, &projectData); err != nil {
		t.Fatalf("Failed to parse projects: %v", err)
	}

	if len(projectData.Projects) == 0 {
		t.Fatal("No projects found")
	}

	// Get first two projects for testing
	var testProjects []string
	for i, p := range projectData.Projects {
		if i >= 2 {
			break
		}
		if key, ok := p["key"].(string); ok {
			testProjects = append(testProjects, key)
			t.Logf("✓ Found project: %s", key)
		}
	}

	if len(testProjects) < 2 {
		t.Fatal("Need at least 2 projects for testing")
	}

	// Step 4: Fetch issues for all projects
	t.Log("Step 4: Fetching issues for all projects...")
	allProjectsJSON := `{"projectKeys":["` + strings.Join(testProjects, `","`) + `"]}`
	resp, err = http.Post(baseURL+"/api/projects/get-issues", "application/json", strings.NewReader(allProjectsJSON))
	if err != nil {
		t.Fatalf("Failed to fetch issues: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Get issues failed with status: %d", resp.StatusCode)
	}
	t.Log("✓ Issue fetching started")
	time.Sleep(15 * time.Second) // Wait for issues to be scraped

	// Step 5: Test filtering for each project
	t.Log("Step 5: Testing issue filtering for each project...")
	for _, projectKey := range testProjects {
		t.Run(fmt.Sprintf("Project_%s", projectKey), func(t *testing.T) {
			testProjectFiltering(t, projectKey)
		})
	}
}

func testProjectFiltering(t *testing.T, projectKey string) {
	url := fmt.Sprintf("%s/api/data/jira/issues?projectKey=%s", baseURL, projectKey)
	t.Logf("Testing: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to get issues for %s: %v", projectKey, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var issueData struct {
		Issues []map[string]interface{} `json:"issues"`
	}
	if err := json.Unmarshal(body, &issueData); err != nil {
		t.Fatalf("Failed to parse issues: %v", err)
	}

	t.Logf("Received %d issues for project %s", len(issueData.Issues), projectKey)

	if len(issueData.Issues) == 0 {
		t.Logf("⚠ No issues returned for project %s (project may be empty)", projectKey)
		return
	}

	// Verify each issue belongs to the correct project
	wrongProjectIssues := 0
	for i, issue := range issueData.Issues {
		issueKey := "unknown"
		if key, ok := issue["key"].(string); ok {
			issueKey = key
		}

		// Check project key in issue fields
		actualProjectKey := ""
		if fields, ok := issue["fields"].(map[string]interface{}); ok {
			if project, ok := fields["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok {
					actualProjectKey = key
				}
			}
		}

		if actualProjectKey != projectKey {
			wrongProjectIssues++
			t.Errorf("❌ Issue %d (%s) has wrong project key: expected %s, got %s",
				i+1, issueKey, projectKey, actualProjectKey)
		}
	}

	if wrongProjectIssues > 0 {
		t.Fatalf("FAILED: %d out of %d issues have wrong project key", wrongProjectIssues, len(issueData.Issues))
	}

	t.Logf("✓ SUCCESS: All %d issues belong to project %s", len(issueData.Issues), projectKey)
}
