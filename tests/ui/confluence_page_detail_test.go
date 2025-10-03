package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestConfluence_PageDetail verifies page selection, detail display, URL persistence, and refresh
func TestConfluence_PageDetail(t *testing.T) {
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

	// Wait for pages to be available in table
	t.Log("Waiting for pages in table...")

	var pagesAvailable bool
	for i := 0; i < 15; i++ {
		var hasPages bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const tbody = document.getElementById('pages-table-body');
					if (!tbody) return false;

					const rows = tbody.querySelectorAll('tr');
					if (rows.length === 0) return false;

					const firstCell = rows[0].querySelector('td');
					if (firstCell && firstCell.colSpan > 1) return false;

					return rows.length > 0;
				})()
			`, &hasPages),
		)
		if err == nil && hasPages {
			pagesAvailable = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !pagesAvailable {
		takeScreenshot(ctx, t, "FAIL_no_pages")
		t.Fatal("❌ No pages available in table (run get pages test first)")
	}

	t.Log("✓ Pages available in table")

	// Click first page row
	t.Log("Clicking first page row...")

	var pageInfo struct {
		PageID    string
		PageTitle string
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const firstRow = tbody.querySelector('tr');

				if (!firstRow) return null;

				const cells = firstRow.querySelectorAll('td');
				if (cells.length < 4) return null;

				const pageId = cells[1].textContent.trim();
				const pageTitle = cells[2].textContent.trim();

				firstRow.click();

				return {
					pageId: pageId,
					pageTitle: pageTitle
				};
			})()
		`, &pageInfo),
	)
	if err != nil || pageInfo.PageID == "" {
		takeScreenshot(ctx, t, "FAIL_page_click")
		t.Fatalf("❌ Failed to click page: %v", err)
	}

	t.Logf("✓ Clicked page: %s (%s)", pageInfo.PageID, pageInfo.PageTitle)
	takeScreenshot(ctx, t, "page_clicked")

	// Wait for page detail to load
	t.Log("Waiting for page detail to display...")

	var detailDisplayed bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			(() => {
				const detailPre = document.getElementById('page-detail-json');
				const selectedId = document.getElementById('selected-page-id');

				if (!detailPre || !selectedId) return false;

				const codeBlock = detailPre.querySelector('code');
				if (!codeBlock) return false;

				const hasContent = codeBlock.textContent.trim().length > 0;
				const hasSelectedId = selectedId.textContent.trim().length > 0;

				return hasContent && hasSelectedId;
			})()
		`, &detailDisplayed),
	)
	if err != nil || !detailDisplayed {
		takeScreenshot(ctx, t, "FAIL_detail_not_displayed")
		t.Fatal("❌ Page detail did not display")
	}

	t.Log("✓ Page detail displayed")
	takeScreenshot(ctx, t, "detail_displayed")

	// Verify page ID is shown in header
	t.Log("Verifying page ID in detail header...")

	var headerPageID string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const selectedId = document.getElementById('selected-page-id');
				return selectedId ? selectedId.textContent.trim() : '';
			})()
		`, &headerPageID),
	)
	if err != nil || headerPageID != pageInfo.PageID {
		takeScreenshot(ctx, t, "FAIL_wrong_page_id")
		t.Fatalf("❌ Wrong page ID in header: expected %s, got %s", pageInfo.PageID, headerPageID)
	}

	t.Logf("✓ Correct page ID in header: %s", headerPageID)

	// Verify JSON content is valid and contains page data
	t.Log("Verifying JSON content...")

	var jsonValid bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const codeBlock = document.querySelector('#page-detail-json code');
				if (!codeBlock) return false;

				try {
					const jsonText = codeBlock.textContent;
					const data = JSON.parse(jsonText);

					// Check for expected page properties
					return data.id && data.title && data.type;
				} catch (e) {
					return false;
				}
			})()
		`, &jsonValid),
	)
	if err != nil || !jsonValid {
		takeScreenshot(ctx, t, "FAIL_invalid_json")
		t.Fatal("❌ Page detail JSON is invalid or missing required properties")
	}

	t.Log("✓ Page detail JSON is valid")
	takeScreenshot(ctx, t, "SUCCESS_json_verified")

	// Verify row is highlighted
	t.Log("Verifying selected row is highlighted...")

	var rowHighlighted bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const rows = tbody.querySelectorAll('tr');

				let highlighted = false;
				rows.forEach(row => {
					const bgColor = window.getComputedStyle(row).backgroundColor;
					if (bgColor !== 'rgba(0, 0, 0, 0)' && bgColor !== 'transparent') {
						highlighted = true;
					}
				});

				return highlighted;
			})()
		`, &rowHighlighted),
	)
	if err != nil || !rowHighlighted {
		takeScreenshot(ctx, t, "FAIL_row_not_highlighted")
		t.Fatal("❌ Selected row is not highlighted")
	}

	t.Log("✓ Selected row is highlighted")
	takeScreenshot(ctx, t, "row_highlighted")

	// Verify URL contains page ID
	t.Log("Verifying URL contains page ID...")

	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)
	if err != nil {
		t.Fatalf("❌ Failed to get URL: %v", err)
	}

	t.Logf("Current URL: %s", currentURL)

	// Check if URL contains page parameter
	var urlContainsPage bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const params = new URLSearchParams(window.location.search);
				const pageParam = params.get('page');
				return pageParam !== null && pageParam.length > 0;
			})()
		`, &urlContainsPage),
	)
	if err != nil || !urlContainsPage {
		takeScreenshot(ctx, t, "FAIL_url_no_page_id")
		t.Fatal("❌ URL does not contain page ID parameter")
	}

	t.Log("✓ URL contains page ID")
	takeScreenshot(ctx, t, "url_with_page_id")

	// Test URL persistence: refresh page
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

	// Verify page detail is still displayed
	t.Log("Verifying page detail persisted after refresh...")

	var detailPersisted bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`
			(() => {
				const selectedId = document.getElementById('selected-page-id');
				if (!selectedId) return false;

				const displayedId = selectedId.textContent.trim();
				return displayedId === '`+pageInfo.PageID+`';
			})()
		`, &detailPersisted),
	)
	if err != nil || !detailPersisted {
		takeScreenshot(ctx, t, "FAIL_detail_not_persisted")
		t.Fatal("❌ Page detail did not persist after refresh")
	}

	t.Log("✓ Page detail persisted")

	// Verify row highlighting persisted
	t.Log("Verifying row highlighting persisted...")

	var highlightPersisted bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const tbody = document.getElementById('pages-table-body');
				const rows = tbody.querySelectorAll('tr');

				let highlighted = false;
				rows.forEach(row => {
					const bgColor = window.getComputedStyle(row).backgroundColor;
					if (bgColor === 'rgb(227, 242, 253)' || bgColor.includes('227, 242, 253')) {
						highlighted = true;
					}
				});

				return highlighted;
			})()
		`, &highlightPersisted),
	)
	if err != nil || !highlightPersisted {
		takeScreenshot(ctx, t, "FAIL_highlight_not_persisted")
		t.Fatal("❌ Row highlighting did not persist after refresh")
	}

	t.Log("✓ Row highlighting persisted")
	takeScreenshot(ctx, t, "SUCCESS_persistence_verified")

	t.Log("✅ All page detail and persistence checks passed successfully")
}
