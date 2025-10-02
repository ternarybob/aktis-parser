package ui

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJira_AutoLoadIssues verifies that selecting a project automatically loads issues from local database
func TestJira_AutoLoadIssues(t *testing.T) {
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

	// Find a project with issues that already exist in the database
	// (from the previous test run that fetched issues)
	t.Log("Finding project with issues in database...")

	var result map[string]interface{}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const projects = Array.from(document.querySelectorAll('.project-item'));
			for (const project of projects) {
				const checkbox = project.querySelector('input[type="checkbox"]');
				const issueCountText = project.querySelector('.project-issues')?.textContent || '0 issues';
				const issueCount = parseInt(issueCountText);

				// Find first project with issues (should have been fetched in previous test)
				if (issueCount > 0) {
					return {
						key: checkbox.value,
						count: issueCount
					};
				}
			}
			return null;
		})()
	`, &result)); err != nil {
		t.Fatalf("Failed to evaluate: %v", err)
	}

	if result == nil {
		t.Skip("No project with issues found in database - run TestJira_GetIssues first")
	}

	projectKey, _ := result["key"].(string)
	var expectedIssueCount int
	if count, ok := result["count"].(float64); ok {
		expectedIssueCount = int(count)
	}

	t.Logf("✓ Found project '%s' with %d issues", projectKey, expectedIssueCount)

	// Verify issues table is initially empty
	var initialIssueCount int
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody ? tbody.querySelectorAll('tr') : [];
			let dataRows = 0;
			for (const row of rows) {
				if (row.querySelectorAll('td').length >= 5) {
					dataRows++;
				}
			}
			return dataRows;
		})()
	`, &initialIssueCount))

	if initialIssueCount != 0 {
		t.Logf("Warning: Issues table should be empty initially, but has %d rows", initialIssueCount)
	}

	// Select the project by clicking its checkbox
	checkboxSelector := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, projectKey)
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select project: %v", err)
	}
	t.Log("✓ Project selected")
	takeScreenshot(ctx, t, "02_project_selected")

	// Wait a moment for auto-load to trigger
	time.Sleep(500 * time.Millisecond)

	// Verify issues auto-loaded from database WITHOUT clicking "GET ISSUES"
	var autoLoadedIssueCount int
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody ? tbody.querySelectorAll('tr') : [];
			let dataRows = 0;

			for (const row of rows) {
				const text = row.textContent;
				// Skip loading/empty message rows
				if (!text.includes('Loading') && !text.includes('No issues') && !text.includes('GET ISSUES')) {
					const cells = row.querySelectorAll('td');
					if (cells.length >= 5) {
						dataRows++;
					}
				}
			}

			return dataRows;
		})()
	`, &autoLoadedIssueCount))

	if err != nil {
		t.Fatalf("Failed to count auto-loaded issues: %v", err)
	}

	takeScreenshot(ctx, t, "03_issues_autoloaded")

	t.Logf("\nAuto-Load Verification:")
	t.Logf("  Expected: %d issues (from project count)", expectedIssueCount)
	t.Logf("  Auto-loaded: %d issues (from local database)", autoLoadedIssueCount)

	if autoLoadedIssueCount == 0 {
		takeScreenshot(ctx, t, "FAIL_no_autoload")
		t.Fatalf("❌ Issues did not auto-load from database when project was selected")
	}

	if autoLoadedIssueCount != expectedIssueCount {
		t.Logf("⚠️  Issue count mismatch: expected %d, got %d", expectedIssueCount, autoLoadedIssueCount)
	} else {
		t.Logf("✓ Issue count matches: %d issues auto-loaded", autoLoadedIssueCount)
	}

	// Verify issues belong to selected project
	var projectKeys []string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const tbody = document.getElementById('issues-table-body');
			const rows = tbody.querySelectorAll('tr');
			const keys = new Set();

			for (const row of rows) {
				const cells = row.querySelectorAll('td');
				if (cells.length >= 5) {
					const projectKey = cells[0].textContent.trim();
					if (projectKey && projectKey !== 'N/A') {
						keys.add(projectKey);
					}
				}
			}

			return Array.from(keys);
		})()
	`, &projectKeys)); err != nil {
		t.Errorf("Failed to extract project keys from issues: %v", err)
	} else {
		t.Logf("\nProject Keys in Issues Table: %v", projectKeys)

		allMatch := true
		for _, key := range projectKeys {
			if key != projectKey {
				t.Errorf("❌ Found issue from unexpected project: %s (expected %s)", key, projectKey)
				allMatch = false
			}
		}

		if allMatch && len(projectKeys) > 0 {
			t.Logf("✓ All issues belong to project '%s'", projectKey)
		}
	}

	takeScreenshot(ctx, t, "04_SUCCESS_autoload_verified")
	t.Log("✅ Auto-load from database verified successfully")
}
