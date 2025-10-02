package ui

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJira_SwitchProjects verifies that switching between projects correctly updates the issues display
func TestJira_SwitchProjects(t *testing.T) {
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}

	// Setup browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Start video recording
	stopRecording, err := startVideoRecording(ctx, t)
	if err != nil {
		t.Logf("Warning: Could not start video recording: %v", err)
	} else {
		defer stopRecording()
	}

	// Navigate to jira page
	t.Log("Navigating to", serverURL+"/jira...")
	if err := chromedp.Run(ctx, chromedp.Navigate(serverURL+"/jira")); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ctx, chromedp.WaitVisible(`#project-list`, chromedp.ByID)); err != nil {
		t.Fatalf("Page did not load: %v", err)
	}
	t.Log("✓ Jira page loaded successfully")
	takeScreenshot(ctx, t, "01_page_loaded")

	// Wait for projects to load
	t.Log("Waiting for projects to load...")
	var projectsLoaded bool
	for i := 0; i < 10; i++ {
		var hasProjects bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const projects = document.querySelectorAll('.project-item');
				return projects.length > 0;
			})()
		`, &hasProjects))

		if hasProjects {
			projectsLoaded = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !projectsLoaded {
		takeScreenshot(ctx, t, "FAIL_no_projects_loaded")
		t.Fatalf("No projects loaded after 10 seconds")
	}
	t.Log("✓ Projects loaded")

	// Find TWO projects with issues in database
	t.Log("Finding two projects with issues...")

	var projects []map[string]interface{}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const projectItems = Array.from(document.querySelectorAll('.project-item'));
			const projectsWithIssues = [];

			for (const project of projectItems) {
				const checkbox = project.querySelector('input[type="checkbox"]');
				const issueCountText = project.querySelector('.project-issues')?.textContent || '0 issues';
				const issueCount = parseInt(issueCountText);

				if (issueCount > 0) {
					projectsWithIssues.push({
						key: checkbox.value,
						count: issueCount
					});
				}
			}

			return projectsWithIssues;
		})()
	`, &projects)); err != nil {
		t.Fatalf("Failed to find projects: %v", err)
	}

	if len(projects) < 2 {
		t.Skipf("Need at least 2 projects with issues, found %d - run TestJira_GetIssues for multiple projects first", len(projects))
	}

	project1Key, _ := projects[0]["key"].(string)
	project1Count := int(projects[0]["count"].(float64))

	project2Key, _ := projects[1]["key"].(string)
	project2Count := int(projects[1]["count"].(float64))

	t.Logf("✓ Found project 1: '%s' with %d issues", project1Key, project1Count)
	t.Logf("✓ Found project 2: '%s' with %d issues", project2Key, project2Count)

	// First, ensure BOTH projects have issues in the database by fetching them
	t.Log("Ensuring both projects have issues in database...")

	// Select both projects
	checkboxSelector1 := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, project1Key)
	chromedp.Run(ctx, chromedp.Click(checkboxSelector1, chromedp.ByQuery))

	checkboxSelector2 := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, project2Key)
	chromedp.Run(ctx, chromedp.Click(checkboxSelector2, chromedp.ByQuery))

	time.Sleep(300 * time.Millisecond)

	// Click GET ISSUES to fetch both
	chromedp.Run(ctx, chromedp.Click(`#get-issues-btn`, chromedp.ByID))
	t.Log("Fetching issues for both projects...")
	time.Sleep(5 * time.Second) // Wait for fetch to complete

	// Deselect all
	chromedp.Run(ctx, chromedp.Click(checkboxSelector1, chromedp.ByQuery))
	chromedp.Run(ctx, chromedp.Click(checkboxSelector2, chromedp.ByQuery))
	time.Sleep(500 * time.Millisecond)

	t.Log("✓ Both projects now have issues in database")

	// Now test switching between projects
	// Select first project
	t.Logf("Selecting first project: %s", project1Key)
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector1, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select project 1: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	takeScreenshot(ctx, t, "02_project1_selected")

	// Verify first project's issues loaded
	var issuesForProject1 []string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody.querySelectorAll('tr');
			const projectKeys = [];

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 5) {
					const projectKey = cells[0].textContent.trim();
					if (projectKey && projectKey !== 'N/A') {
						projectKeys.push(projectKey);
					}
				}
			}

			return projectKeys;
		})()
	`, &issuesForProject1))

	if len(issuesForProject1) == 0 {
		takeScreenshot(ctx, t, "FAIL_no_issues_for_project1")
		t.Fatalf("❌ No issues loaded for project 1: %s", project1Key)
	}

	// Verify all issues belong to project 1
	allMatch := true
	for _, key := range issuesForProject1 {
		if key != project1Key {
			t.Errorf("❌ Expected issues from %s, but found issue from %s", project1Key, key)
			allMatch = false
		}
	}

	if allMatch {
		t.Logf("✓ All %d issues belong to project 1: %s", len(issuesForProject1), project1Key)
	}

	// NOW SWITCH TO PROJECT 2
	t.Logf("\nSwitching to second project: %s", project2Key)

	// Deselect project 1
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector1, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to deselect project 1: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Select project 2
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector2, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select project 2: %v", err)
	}
	time.Sleep(1000 * time.Millisecond) // Give more time for async load

	takeScreenshot(ctx, t, "03_project2_selected")

	// Verify second project's issues loaded (NOT project 1's issues!)
	var issuesForProject2 []string
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody.querySelectorAll('tr');
			const projectKeys = [];

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 5) {
					const projectKey = cells[0].textContent.trim();
					if (projectKey && projectKey !== 'N/A') {
						projectKeys.push(projectKey);
					}
				}
			}

			return projectKeys;
		})()
	`, &issuesForProject2))

	if len(issuesForProject2) == 0 {
		takeScreenshot(ctx, t, "FAIL_no_issues_for_project2")
		t.Fatalf("❌ No issues loaded for project 2: %s", project2Key)
	}

	// CRITICAL: Verify all issues belong to project 2 (NOT project 1!)
	allMatchProject2 := true
	wrongProjectFound := false
	for _, key := range issuesForProject2 {
		if key != project2Key {
			if key == project1Key {
				t.Errorf("❌ CRITICAL: Found issue from PROJECT 1 (%s) when PROJECT 2 (%s) is selected! Poll cancellation failed!", project1Key, project2Key)
				wrongProjectFound = true
			} else {
				t.Errorf("❌ Expected issues from %s, but found issue from %s", project2Key, key)
			}
			allMatchProject2 = false
		}
	}

	if wrongProjectFound {
		takeScreenshot(ctx, t, "FAIL_wrong_project_issues")
		t.Fatalf("❌ Project switching failed - showing wrong project's issues")
	}

	if allMatchProject2 {
		t.Logf("✓ All %d issues belong to project 2: %s", len(issuesForProject2), project2Key)
	}

	takeScreenshot(ctx, t, "04_SUCCESS_project_switching_works")
	t.Log("✅ Project switching verified successfully - issues update correctly")
}
