package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestConfluence_SyncSpaces verifies that syncing spaces correctly retrieves page counts
func TestConfluence_SyncSpaces(t *testing.T) {
	screenshotCounter = 0

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer ctxCancel()

	stopRecording, err := startVideoRecording(ctx, t)
	if err != nil {
		t.Logf("Warning: Could not start video recording: %v", err)
	} else {
		defer stopRecording()
	}

	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}
	confluenceURL := serverURL + "/confluence"

	t.Logf("Navigating to %s...", confluenceURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(confluenceURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_page_load")
		t.Fatalf("❌ Failed to load Confluence page: %v", err)
	}

	t.Log("✓ Confluence page loaded successfully")
	takeScreenshot(ctx, t, "confluence_page_loaded")

	// Clear data first
	t.Log("Clearing data first...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = () => true`, nil),
		chromedp.Click(`#clear-data-btn`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		t.Logf("Warning: Could not clear data: %v", err)
	}

	// Check for GET SPACES button
	t.Log("Checking for GET SPACES button...")

	var syncButtonExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const syncBtn = document.getElementById('sync-btn');
				return syncBtn !== null;
			})()
		`, &syncButtonExists),
	)
	if err != nil || !syncButtonExists {
		takeScreenshot(ctx, t, "FAIL_sync_button_not_found")
		t.Fatalf("❌ GET SPACES button not found")
	}

	t.Log("✓ GET SPACES button found")

	// Click GET SPACES button
	t.Log("Clicking GET SPACES button...")

	err = chromedp.Run(ctx,
		chromedp.Click(`#sync-btn`, chromedp.ByQuery),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_sync_button_click")
		t.Fatalf("❌ Failed to click GET SPACES button: %v", err)
	}

	t.Log("✓ GET SPACES button clicked")
	takeScreenshot(ctx, t, "sync_button_clicked")

	// Wait for sync to complete (poll for spaces to appear WITH page counts)
	t.Log("Waiting for spaces to sync with page counts (max 30 seconds)...")

	var syncComplete bool
	maxWait := 30 * time.Second
	pollInterval := 3 * time.Second
	startTime := time.Now()

	time.Sleep(5 * time.Second)

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasSpaces        bool
			SpaceCount       int
			HasAnyPages      bool
			SyncMsgDisplayed bool
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const spaceList = document.getElementById('space-list');
					if (!spaceList) return { hasSpaces: false, spaceCount: 0, hasAnyPages: false, syncMsgDisplayed: false };

					const items = spaceList.querySelectorAll('.project-item');
					if (items.length === 0) return { hasSpaces: false, spaceCount: 0, hasAnyPages: false, syncMsgDisplayed: false };

					// Check if any space has pages
					let hasPages = false;
					items.forEach(item => {
						const pagesSpan = item.querySelector('.project-issues');
						if (pagesSpan) {
							const text = pagesSpan.textContent.trim();
							const pageCount = parseInt(text.match(/\d+/)?.[0] || '0');
							if (pageCount > 0) {
								hasPages = true;
							}
						}
					});

					const notification = document.getElementById('sync-notification');
					const syncMsgDisplayed = notification && notification.textContent.includes('Successfully loaded');

					return {
						hasSpaces: true,
						spaceCount: items.length,
						hasAnyPages: hasPages,
						syncMsgDisplayed: syncMsgDisplayed
					};
				})()
			`, &checkResult),
		)

		if err == nil && checkResult.HasSpaces && checkResult.HasAnyPages {
			syncComplete = true
			t.Logf("Sync complete: %d spaces, at least one has pages", checkResult.SpaceCount)
			break
		}

		if err == nil {
			t.Logf("Waiting... spaces=%d, hasPages=%v, elapsed=%v",
				checkResult.SpaceCount, checkResult.HasAnyPages, time.Since(startTime).Round(time.Second))
		}

		time.Sleep(pollInterval)
	}

	if !syncComplete {
		takeScreenshot(ctx, t, "FAIL_sync_timeout")
		t.Fatalf("❌ Sync did not complete with page counts within %v", maxWait)
	}

	t.Logf("✓ Sync completed after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "sync_completed")

	// Check that NOT all spaces have 0 pages
	t.Log("Verifying that at least one space has pages...")

	var hasSpacesWithPages bool
	var spaceStats map[string]interface{}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const spaceItems = document.querySelectorAll('.project-item');
				let total = 0;
				let withPages = 0;
				let withZeroPages = 0;
				const spaces = [];

				spaceItems.forEach(item => {
					const pagesSpan = item.querySelector('.project-issues');
					const keySpan = item.querySelector('.project-key');

					if (pagesSpan && keySpan) {
						total++;
						const text = pagesSpan.textContent.trim();
						const pageCount = parseInt(text.match(/\d+/)?.[0] || '0');

						spaces.push({
							key: keySpan.textContent.trim(),
							pageCount: pageCount,
							text: text
						});

						if (pageCount > 0) {
							withPages++;
						} else {
							withZeroPages++;
						}
					}
				});

				return {
					total: total,
					withPages: withPages,
					withZeroPages: withZeroPages,
					hasSpacesWithPages: withPages > 0,
					spaces: spaces
				};
			})()
		`, &spaceStats),
	)

	if err != nil {
		takeScreenshot(ctx, t, "FAIL_page_count_check")
		t.Fatalf("❌ Failed to check page counts: %v", err)
	}

	// Log detailed statistics
	t.Logf("Space Statistics:")
	t.Logf("  Total spaces: %.0f", spaceStats["total"])
	t.Logf("  Spaces with pages: %.0f", spaceStats["withPages"])
	t.Logf("  Spaces with 0 pages: %.0f", spaceStats["withZeroPages"])

	// Log individual spaces
	if spaces, ok := spaceStats["spaces"].([]interface{}); ok {
		t.Log("  Individual spaces:")
		for _, s := range spaces {
			if space, ok := s.(map[string]interface{}); ok {
				key := space["key"]
				count := space["pageCount"]
				text := space["text"]
				t.Logf("    - %v: %v (%v)", key, count, text)
			}
		}
	}

	hasSpacesWithPages = spaceStats["hasSpacesWithPages"].(bool)

	if !hasSpacesWithPages {
		takeScreenshot(ctx, t, "FAIL_all_spaces_zero_pages")
		t.Fatalf("❌ FAILURE: All spaces have 0 pages! This indicates the page count API is not working correctly.")
	}

	t.Logf("✓ At least one space has pages (%.0f spaces with pages)", spaceStats["withPages"])
	takeScreenshot(ctx, t, "SUCCESS_page_counts_verified")

	takeScreenshot(ctx, t, "SUCCESS_all_checks_passed")
	t.Log("✅ All Confluence sync checks passed successfully")
}
