package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJira_SyncProjects verifies that syncing projects correctly retrieves issue counts
func TestJira_SyncProjects(t *testing.T) {
	screenshotCounter = 0

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer ctxCancel()

	// Start video recording
	stopRecording, err := startVideoRecording(ctx, t)
	if err != nil {
		t.Logf("Warning: Could not start video recording: %v", err)
	} else {
		defer stopRecording()
	}

	// Navigate to the Jira page
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}
	jiraURL := serverURL + "/jira"

	t.Logf("Navigating to %s...", jiraURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(jiraURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_jira_page_load")
		t.Fatalf("❌ Failed to load Jira page: %v", err)
	}

	t.Log("✓ Jira page loaded successfully")
	takeScreenshot(ctx, t, "jira_page_loaded")

	// Wait for page to be fully ready
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		t.Fatalf("❌ Failed to wait: %v", err)
	}

	// Check if SYNC PROJECTS button exists
	t.Log("Checking for SYNC PROJECTS button...")

	var syncButtonExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const syncBtn = document.getElementById('sync-btn');
				return syncBtn !== null;
			})()
		`, &syncButtonExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_sync_button_check")
		t.Fatalf("❌ Failed to check for sync button: %v", err)
	}

	if !syncButtonExists {
		takeScreenshot(ctx, t, "FAIL_sync_button_not_found")
		t.Fatalf("❌ SYNC PROJECTS button not found")
	}
	t.Log("✓ SYNC PROJECTS button found")

	// Click SYNC PROJECTS button
	t.Log("Clicking SYNC PROJECTS button...")

	err = chromedp.Run(ctx,
		chromedp.Click(`#sync-btn`, chromedp.ByQuery),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_sync_button_click")
		t.Fatalf("❌ Failed to click SYNC PROJECTS button: %v", err)
	}

	t.Log("✓ SYNC PROJECTS button clicked")
	takeScreenshot(ctx, t, "sync_button_clicked")

	// Wait for sync to complete (poll for projects to appear WITH issue counts)
	t.Log("Waiting for projects to sync with issue counts (max 30 seconds)...")

	var syncComplete bool
	maxWait := 30 * time.Second
	pollInterval := 3 * time.Second
	startTime := time.Now()

	// Wait a minimum time for the sync to actually happen
	time.Sleep(5 * time.Second)

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasProjects      bool
			ProjectCount     int
			HasAnyIssues     bool
			SyncMsgDisplayed bool
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const projectList = document.getElementById('project-list');
					if (!projectList) return { hasProjects: false, projectCount: 0, hasAnyIssues: false, syncMsgDisplayed: false };

					const items = projectList.querySelectorAll('.project-item');
					if (items.length === 0) return { hasProjects: false, projectCount: 0, hasAnyIssues: false, syncMsgDisplayed: false };

					// Check if any project has issues
					let hasIssues = false;
					items.forEach(item => {
						const issuesSpan = item.querySelector('.project-issues');
						if (issuesSpan) {
							const text = issuesSpan.textContent.trim();
							const issueCount = parseInt(text.match(/\d+/)?.[0] || '0');
							if (issueCount > 0) {
								hasIssues = true;
							}
						}
					});

					// Check for sync completion message
					const notification = document.getElementById('sync-notification');
					const syncMsgDisplayed = notification && notification.textContent.includes('Successfully loaded');

					return {
						hasProjects: true,
						projectCount: items.length,
						hasAnyIssues: hasIssues,
						syncMsgDisplayed: syncMsgDisplayed
					};
				})()
			`, &checkResult),
		)

		if err == nil && checkResult.HasProjects && checkResult.HasAnyIssues {
			syncComplete = true
			t.Logf("Sync complete: %d projects, at least one has issues", checkResult.ProjectCount)
			break
		}

		if err == nil {
			t.Logf("Waiting... projects=%d, hasIssues=%v, elapsed=%v",
				checkResult.ProjectCount, checkResult.HasAnyIssues, time.Since(startTime).Round(time.Second))
		}

		time.Sleep(pollInterval)
	}

	if !syncComplete {
		takeScreenshot(ctx, t, "FAIL_sync_timeout")
		t.Fatalf("❌ Sync did not complete with issue counts within %v", maxWait)
	}

	t.Logf("✓ Sync completed after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "sync_completed")

	// Check that NOT all projects have 0 issues
	t.Log("Verifying that at least one project has issues...")

	var hasProjectsWithIssues bool
	var projectStats map[string]interface{}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const projectItems = document.querySelectorAll('.project-item');
				let total = 0;
				let withIssues = 0;
				let withZeroIssues = 0;
				const projects = [];

				projectItems.forEach(item => {
					const issuesSpan = item.querySelector('.project-issues');
					const keySpan = item.querySelector('.project-key');

					if (issuesSpan && keySpan) {
						total++;
						const text = issuesSpan.textContent.trim();
						const issueCount = parseInt(text.match(/\d+/)?.[0] || '0');

						projects.push({
							key: keySpan.textContent.trim(),
							issueCount: issueCount,
							text: text
						});

						if (issueCount > 0) {
							withIssues++;
						} else {
							withZeroIssues++;
						}
					}
				});

				return {
					total: total,
					withIssues: withIssues,
					withZeroIssues: withZeroIssues,
					hasProjectsWithIssues: withIssues > 0,
					projects: projects
				};
			})()
		`, &projectStats),
	)

	if err != nil {
		takeScreenshot(ctx, t, "FAIL_issue_count_check")
		t.Fatalf("❌ Failed to check issue counts: %v", err)
	}

	// Log detailed statistics
	t.Logf("Project Statistics:")
	t.Logf("  Total projects: %.0f", projectStats["total"])
	t.Logf("  Projects with issues: %.0f", projectStats["withIssues"])
	t.Logf("  Projects with 0 issues: %.0f", projectStats["withZeroIssues"])

	// Log individual projects
	if projects, ok := projectStats["projects"].([]interface{}); ok {
		t.Log("  Individual projects:")
		for _, p := range projects {
			if proj, ok := p.(map[string]interface{}); ok {
				key := proj["key"]
				count := proj["issueCount"]
				text := proj["text"]
				t.Logf("    - %v: %v (%v)", key, count, text)
			}
		}
	}

	hasProjectsWithIssues = projectStats["hasProjectsWithIssues"].(bool)

	if !hasProjectsWithIssues {
		takeScreenshot(ctx, t, "FAIL_all_projects_zero_issues")
		t.Fatalf("❌ FAILURE: All projects have 0 issues! This indicates the issue count API is not working correctly.")
	}

	t.Logf("✓ At least one project has issues (%.0f projects with issues)", projectStats["withIssues"])
	takeScreenshot(ctx, t, "SUCCESS_issue_counts_verified")

	// Final success screenshot
	takeScreenshot(ctx, t, "SUCCESS_all_checks_passed")
	t.Log("✅ All Jira sync checks passed successfully")
}
