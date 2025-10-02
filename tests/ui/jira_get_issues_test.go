package ui

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestJira_GetIssues(t *testing.T) {
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
	takeScreenshot(ctx, t, "01_jira_page_loaded")

	// Wait for projects to load from API
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

	// Find a project with issues (issue count > 0)
	t.Log("Finding project with issues...")

	// First, log what we're seeing
	var debug map[string]interface{}
	chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const projects = Array.from(document.querySelectorAll('.project-item'));
			return {
				count: projects.length,
				samples: projects.slice(0, 3).map(p => ({
					checkbox: p.querySelector('input[type="checkbox"]')?.value,
					issueText: p.querySelector('.project-issues')?.textContent
				}))
			};
		})()
	`, &debug))
	t.Logf("Debug - Found %v projects, samples: %+v", debug["count"], debug["samples"])

	var result map[string]interface{}

	if err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const projects = Array.from(document.querySelectorAll('.project-item'));
			for (const project of projects) {
				const checkbox = project.querySelector('input[type="checkbox"]');
				const issueCountText = project.querySelector('.project-issues')?.textContent || '0 issues';
				const issueCount = parseInt(issueCountText);

				if (issueCount > 0 && issueCount <= 100) {
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
		t.Fatalf("No project with issues found")
	}

	projectWithIssues, _ := result["key"].(string)
	var expectedIssueCount int
	if count, ok := result["count"].(float64); ok {
		expectedIssueCount = int(count)
	}

	t.Logf("✓ Found project '%s' with %d issues", projectWithIssues, expectedIssueCount)

	// Select the project by clicking its checkbox
	checkboxSelector := fmt.Sprintf(`input[type="checkbox"][value="%s"]`, projectWithIssues)
	if err := chromedp.Run(ctx, chromedp.Click(checkboxSelector, chromedp.ByQuery)); err != nil {
		t.Fatalf("Failed to select project: %v", err)
	}
	t.Log("✓ Project selected")
	takeScreenshot(ctx, t, "02_project_selected")

	// Click GET ISSUES button
	t.Log("Clicking GET ISSUES button...")
	if err := chromedp.Run(ctx, chromedp.Click(`#get-issues-btn`, chromedp.ByID)); err != nil {
		t.Fatalf("Failed to click GET ISSUES button: %v", err)
	}
	t.Log("✓ GET ISSUES button clicked")
	takeScreenshot(ctx, t, "03_get_issues_clicked")

	// Check what happened after clicking the button
	time.Sleep(3 * time.Second)
	var apiDebug map[string]interface{}
	chromedp.Run(ctx, chromedp.Evaluate(`
		(async () => {
			// Check what the fetch API call returned
			try {
				const projectKeys = ['BI9LLQNGKQ'];
				const response = await fetch('/api/projects/get-issues', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify({ projectKeys: projectKeys })
				});
				const result = await response.json();

				// Also check if issues exist in DB
				const issuesResp = await fetch('/api/data/jira/issues?projectKey=BI9LLQNGKQ');
				const issuesData = await issuesResp.json();

				return {
					apiStatus: response.status,
					apiResult: result,
					issuesCount: issuesData.issues ? issuesData.issues.length : 0
				};
			} catch (e) {
				return { error: e.toString() };
			}
		})()
	`, &apiDebug))
	t.Logf("API Debug: %+v", apiDebug)

	// Wait for issues to load (poll for issues to appear in table)
	t.Log("Waiting for issues to load (max 30 seconds)...")
	var issuesLoaded bool
	var actualIssueCount int
	maxWait := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasIssues    bool
			IssueCount   int
			TableHasRows bool
		}

		err := chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('issues-table-body');
				const rows = tbody ? tbody.querySelectorAll('tr') : [];

				// Check if we have data rows (not loading/empty message)
				let dataRows = 0;
				let hasLoadingMessage = false;

				for (const row of rows) {
					const text = row.textContent;
					if (text.includes('Loading') || text.includes('No issues')) {
						hasLoadingMessage = true;
					} else if (row.querySelectorAll('td').length >= 5) {
						// Has proper columns, count as data row
						dataRows++;
					}
				}

				return {
					HasIssues: dataRows > 0,
					IssueCount: dataRows,
					TableHasRows: rows.length > 0
				};
			})()
		`, &checkResult))

		if err != nil {
			t.Logf("Error checking issues: %v", err)
		} else {
			t.Logf("Waiting... issues=%d, hasIssues=%v, elapsed=%v",
				checkResult.IssueCount, checkResult.HasIssues, time.Since(startTime).Round(time.Second))

			if checkResult.HasIssues && checkResult.IssueCount > 0 {
				issuesLoaded = true
				actualIssueCount = checkResult.IssueCount
				break
			}
		}

		time.Sleep(1 * time.Second)
	}

	if !issuesLoaded {
		takeScreenshot(ctx, t, "04_FAIL_no_issues_loaded")
		t.Fatalf("❌ Issues did not load within 30 seconds")
	}

	t.Logf("✓ Issues loaded after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "05_issues_loaded")

	// Verify issue count matches expected
	t.Logf("\nIssue Count Verification:")
	t.Logf("  Expected: %d issues (from project count)", expectedIssueCount)
	t.Logf("  Actual: %d issues (loaded in table)", actualIssueCount)

	if actualIssueCount != expectedIssueCount {
		takeScreenshot(ctx, t, "06_FAIL_issue_count_mismatch")
		t.Errorf("❌ Issue count mismatch: expected %d, got %d", expectedIssueCount, actualIssueCount)
	} else {
		t.Logf("✓ Issue count matches: %d issues", actualIssueCount)
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

		// Verify all issues belong to selected project
		allMatch := true
		for _, key := range projectKeys {
			if key != projectWithIssues {
				t.Errorf("❌ Found issue from unexpected project: %s (expected %s)", key, projectWithIssues)
				allMatch = false
			}
		}

		if allMatch && len(projectKeys) > 0 {
			t.Logf("✓ All issues belong to project '%s'", projectWithIssues)
		}
	}

	takeScreenshot(ctx, t, "07_SUCCESS_all_checks_passed")
	t.Log("✅ All Get Issues checks passed successfully")
}
