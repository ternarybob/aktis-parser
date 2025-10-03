package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestConfluence_ClearAllData verifies clearing all data works correctly
func TestConfluence_ClearAllData(t *testing.T) {
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

	// Check for CLEAR ALL DATA button
	t.Log("Checking for CLEAR ALL DATA button...")

	var clearButtonExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const clearBtn = document.getElementById('clear-data-btn');
				return clearBtn !== null;
			})()
		`, &clearButtonExists),
	)
	if err != nil || !clearButtonExists {
		takeScreenshot(ctx, t, "FAIL_clear_button_not_found")
		t.Fatalf("❌ CLEAR ALL DATA button not found")
	}

	t.Log("✓ CLEAR ALL DATA button found")

	// Override window.confirm BEFORE clicking to auto-accept dialogs
	t.Log("Setting up auto-confirm for dialogs...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = () => true;`, nil),
	)
	if err != nil {
		t.Fatalf("❌ Failed to override window.confirm: %v", err)
	}

	// Click CLEAR ALL DATA button (dialog will be auto-accepted)
	t.Log("Clicking CLEAR ALL DATA button...")
	err = chromedp.Run(ctx,
		chromedp.Click(`#clear-data-btn`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_clear_button_click")
		t.Fatalf("❌ Failed to click CLEAR ALL DATA button: %v", err)
	}

	t.Log("✓ Confirmation auto-accepted and data cleared")
	takeScreenshot(ctx, t, "clear_data_clicked")

	// Verify data is cleared
	t.Log("Verifying data is cleared...")

	var dataCleared struct {
		SpacesEmpty bool
		PagesEmpty  bool
	}

	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			(() => {
				const spaceList = document.getElementById('space-list');
				const pagesTable = document.getElementById('pages-table-body');

				const spacesText = spaceList ? spaceList.textContent : '';
				const pagesText = pagesTable ? pagesTable.textContent : '';

				return {
					spacesEmpty: spacesText.includes('No spaces in cache') || spacesText.includes('Loading'),
					pagesEmpty: pagesText.includes('No pages loaded')
				};
			})()
		`, &dataCleared),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_verify_cleared")
		t.Fatalf("❌ Failed to verify data cleared: %v", err)
	}

	if !dataCleared.SpacesEmpty {
		takeScreenshot(ctx, t, "FAIL_spaces_not_cleared")
		t.Fatalf("❌ Spaces were not cleared")
	}

	if !dataCleared.PagesEmpty {
		takeScreenshot(ctx, t, "FAIL_pages_not_cleared")
		t.Fatalf("❌ Pages were not cleared")
	}

	t.Log("✓ Data cleared successfully")
	takeScreenshot(ctx, t, "SUCCESS_data_cleared")

	t.Log("✅ Clear all data UI test passed")
}
