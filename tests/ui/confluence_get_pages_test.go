package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestConfluence_GetPages verifies selecting space, getting pages, URL persistence, and refresh
func TestConfluence_GetPages(t *testing.T) {
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
	takeScreenshot(ctx, t, "page_loaded")

	// Wait for spaces to load
	t.Log("Waiting for spaces to load...")

	var spacesLoaded bool
	for i := 0; i < 10; i++ {
		var hasSpaces bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const spaceList = document.getElementById('space-list');
					if (!spaceList) return false;
					const items = spaceList.querySelectorAll('.project-item');
					return items.length > 0;
				})()
			`, &hasSpaces),
		)
		if err == nil && hasSpaces {
			spacesLoaded = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !spacesLoaded {
		takeScreenshot(ctx, t, "FAIL_no_spaces")
		t.Fatal("❌ No spaces loaded (run sync test first)")
	}

	t.Log("✓ Spaces loaded")

	// Select first space
	t.Log("Selecting first space...")

	var spaceKey string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const firstSpace = document.querySelector('.project-item');
				if (!firstSpace) return null;

				const keySpan = firstSpace.querySelector('.project-key');
				const spaceKey = keySpan ? keySpan.textContent.trim() : null;

				firstSpace.click();
				return spaceKey;
			})()
		`, &spaceKey),
	)
	if err != nil || spaceKey == "" {
		takeScreenshot(ctx, t, "FAIL_space_selection")
		t.Fatalf("❌ Failed to select space: %v", err)
	}

	t.Logf("✓ Selected space: %s", spaceKey)
	takeScreenshot(ctx, t, "space_selected")

	// Verify URL contains space key
	t.Log("Verifying URL contains space key...")

	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)
	if err != nil {
		t.Fatalf("❌ Failed to get URL: %v", err)
	}

	if currentURL == "" || currentURL == confluenceURL {
		takeScreenshot(ctx, t, "FAIL_url_not_updated")
		t.Fatalf("❌ URL was not updated with space selection")
	}

	t.Logf("✓ URL updated: %s", currentURL)
	takeScreenshot(ctx, t, "url_with_space")

	// Click GET PAGES button
	t.Log("Clicking GET PAGES button...")

	err = chromedp.Run(ctx,
		chromedp.Click(`#get-pages-menu-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_get_pages_click")
		t.Fatalf("❌ Failed to click GET PAGES button: %v", err)
	}

	t.Log("✓ GET PAGES button clicked")
	takeScreenshot(ctx, t, "get_pages_clicked")

	// Wait for pages to load
	t.Log("Waiting for pages to load (max 30 seconds)...")

	var pagesLoaded bool
	maxWait := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		var checkResult struct {
			HasPages   bool
			PageCount  int
			SpaceMatch bool
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const tbody = document.getElementById('pages-table-body');
					if (!tbody) return { hasPages: false, pageCount: 0, spaceMatch: false };

					const rows = tbody.querySelectorAll('tr');
					if (rows.length === 0) return { hasPages: false, pageCount: 0, spaceMatch: false };

					// Check if showing empty state
					const firstCell = rows[0].querySelector('td');
					if (firstCell && firstCell.colSpan > 1) {
						return { hasPages: false, pageCount: 0, spaceMatch: false };
					}

					// Check if pages match selected space
					let spaceMatch = false;
					rows.forEach(row => {
						const cells = row.querySelectorAll('td');
						if (cells.length >= 4) {
							const pageSpace = cells[0].textContent.trim();
							if (pageSpace === '`+spaceKey+`') {
								spaceMatch = true;
							}
						}
					});

					return {
						hasPages: rows.length > 0,
						pageCount: rows.length,
						spaceMatch: spaceMatch
					};
				})()
			`, &checkResult),
		)

		if err == nil && checkResult.HasPages && checkResult.SpaceMatch {
			pagesLoaded = true
			t.Logf("Pages loaded: %d pages for space %s", checkResult.PageCount, spaceKey)
			break
		}

		if err == nil {
			t.Logf("Waiting... pages=%d, spaceMatch=%v, elapsed=%v",
				checkResult.PageCount, checkResult.SpaceMatch, time.Since(startTime).Round(time.Second))
		}

		time.Sleep(2 * time.Second)
	}

	if !pagesLoaded {
		takeScreenshot(ctx, t, "FAIL_pages_not_loaded")
		t.Fatal("❌ Pages did not load within timeout")
	}

	t.Logf("✓ Pages loaded after %v", time.Since(startTime).Round(time.Second))
	takeScreenshot(ctx, t, "pages_loaded")

	// Verify all pages belong to selected space
	t.Log("Verifying all pages belong to selected space...")

	var allPagesMatch bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const rows = tbody.querySelectorAll('tr');

				let allMatch = true;
				rows.forEach(row => {
					const cells = row.querySelectorAll('td');
					if (cells.length >= 4) {
						const pageSpace = cells[0].textContent.trim();
						if (pageSpace !== '`+spaceKey+`') {
							allMatch = false;
						}
					}
				});

				return allMatch;
			})()
		`, &allPagesMatch),
	)
	if err != nil || !allPagesMatch {
		takeScreenshot(ctx, t, "FAIL_wrong_space_pages")
		t.Fatal("❌ Some pages do not belong to selected space")
	}

	t.Log("✓ All pages match selected space")
	takeScreenshot(ctx, t, "SUCCESS_pages_verified")

	// Test URL persistence: refresh page and verify space/pages reload
	t.Log("Testing URL persistence: refreshing page...")

	err = chromedp.Run(ctx,
		chromedp.Reload(),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_page_refresh")
		t.Fatalf("❌ Failed to refresh page: %v", err)
	}

	t.Log("✓ Page refreshed")
	takeScreenshot(ctx, t, "page_refreshed")

	// Verify space is still selected
	t.Log("Verifying space selection persisted...")

	var spaceStillSelected bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`
			(() => {
				const spaces = document.querySelectorAll('.project-item');
				let selected = false;

				spaces.forEach(space => {
					const keySpan = space.querySelector('.project-key');
					const checkbox = space.querySelector('input[type="checkbox"]');

					if (keySpan && keySpan.textContent.trim() === '`+spaceKey+`') {
						if (checkbox && checkbox.checked) {
							selected = true;
						}
					}
				});

				return selected;
			})()
		`, &spaceStillSelected),
	)
	if err != nil || !spaceStillSelected {
		takeScreenshot(ctx, t, "FAIL_space_not_persisted")
		t.Fatal("❌ Space selection did not persist after refresh")
	}

	t.Log("✓ Space selection persisted")

	// Verify pages are still loaded
	t.Log("Verifying pages persisted...")

	var pagesPersisted bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const rows = tbody.querySelectorAll('tr');

				if (rows.length === 0) return false;

				const firstCell = rows[0].querySelector('td');
				if (firstCell && firstCell.colSpan > 1) return false;

				return rows.length > 0;
			})()
		`, &pagesPersisted),
	)
	if err != nil || !pagesPersisted {
		takeScreenshot(ctx, t, "FAIL_pages_not_persisted")
		t.Fatal("❌ Pages did not persist after refresh")
	}

	t.Log("✓ Pages persisted after refresh")
	takeScreenshot(ctx, t, "SUCCESS_persistence_verified")

	t.Log("✅ All get pages and persistence checks passed successfully")
}
