package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var screenshotCounter int

func takeScreenshot(ctx context.Context, t *testing.T, name string) {
	screenshotCounter++
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	filename := fmt.Sprintf("%02d_%s.png", screenshotCounter, name)
	screenshotPath := filepath.Join(runDir, filename)

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
		os.MkdirAll(filepath.Dir(screenshotPath), 0755)
		if err := os.WriteFile(screenshotPath, buf, 0644); err == nil {
			t.Logf("📸 Screenshot %d: %s", screenshotCounter, filename)
		}
	}
}

func startVideoRecording(ctx context.Context, t *testing.T) (func(), error) {
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	videoPath := filepath.Join(runDir, "test_recording.webm")
	os.MkdirAll(filepath.Dir(videoPath), 0755)

	frameCount := 0
	maxFrames := 300 // 30 seconds at 10fps

	// Start screencast
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("png").
				WithQuality(80).
				WithEveryNthFrame(6). // ~10fps at 60fps base
				Do(ctx)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start screencast: %w", err)
	}

	t.Log("🎥 Video recording started")

	// Cleanup function
	stopRecording := func() {
		chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				return page.StopScreencast().Do(ctx)
			}),
		)
		t.Logf("🎥 Video recording stopped (%d frames captured)", frameCount)
	}

	// Listen for screencast frames
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if frameCount >= maxFrames {
			return
		}

		if _, ok := ev.(*page.EventScreencastFrame); ok {
			frameCount++
		}
	})

	return stopRecording, nil
}

// TestUI_ParserPageLoads verifies that the parser UI page loads correctly
func TestUI_ParserPageLoads(t *testing.T) {
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

	// Navigate to the application
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}

	t.Logf("Navigating to %s...", serverURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_page_load")
		t.Fatalf("❌ Failed to load page: %v", err)
	}

	t.Log("✓ Page loaded successfully")
	takeScreenshot(ctx, t, "page_loaded")

	// Check for navbar title
	t.Log("Checking for navbar title...")

	var navbarExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const navbar = document.querySelector('.navbar-brand-title');
				return navbar !== null && navbar.textContent.includes('AKTIS-PARSER');
			})()
		`, &navbarExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_navbar_check_error")
		t.Fatalf("❌ Failed to check for navbar: %v", err)
	}

	if !navbarExists {
		takeScreenshot(ctx, t, "FAIL_navbar_not_found")
		t.Fatalf("❌ Navbar title 'AKTIS-PARSER' not found in UI")
	}
	t.Log("✓ Navbar title found")

	// Check for status indicator
	t.Log("Checking for status indicator...")

	var statusIndicatorExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const statusIndicator = document.querySelector('.status-indicator');
				return statusIndicator !== null;
			})()
		`, &statusIndicatorExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_status_indicator_error")
		t.Fatalf("❌ Failed to check for status indicator: %v", err)
	}

	if !statusIndicatorExists {
		takeScreenshot(ctx, t, "FAIL_status_indicator_not_found")
		t.Fatalf("❌ Status indicator not found in UI")
	}
	t.Log("✓ Status indicator found")

	// Check for service status section
	t.Log("Checking for SERVICE STATUS section...")

	var serviceStatusExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const headers = Array.from(document.querySelectorAll('h2'));
				return headers.some(h => h.textContent.includes('SERVICE STATUS'));
			})()
		`, &serviceStatusExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_service_status_error")
		t.Fatalf("❌ Failed to check for service status section: %v", err)
	}

	if !serviceStatusExists {
		takeScreenshot(ctx, t, "FAIL_service_status_not_found")
		t.Fatalf("❌ 'SERVICE STATUS' section not found in UI")
	}
	t.Log("✓ 'SERVICE STATUS' section found")

	// Check for scraping status section
	t.Log("Checking for SCRAPING STATUS section...")

	var scrapingStatusExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const headers = Array.from(document.querySelectorAll('h2'));
				return headers.some(h => h.textContent.includes('SCRAPING STATUS'));
			})()
		`, &scrapingStatusExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_scraping_status_error")
		t.Fatalf("❌ Failed to check for scraping status section: %v", err)
	}

	if !scrapingStatusExists {
		takeScreenshot(ctx, t, "FAIL_scraping_status_not_found")
		t.Fatalf("❌ 'SCRAPING STATUS' section not found in UI")
	}
	t.Log("✓ 'SCRAPING STATUS' section found")

	// Check for service logs section
	t.Log("Checking for SERVICE LOGS section...")

	var serviceLogsExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const headers = Array.from(document.querySelectorAll('h2'));
				return headers.some(h => h.textContent.includes('SERVICE LOGS'));
			})()
		`, &serviceLogsExists),
	)
	if err != nil {
		takeScreenshot(ctx, t, "FAIL_service_logs_error")
		t.Fatalf("❌ Failed to check for service logs section: %v", err)
	}

	if !serviceLogsExists {
		takeScreenshot(ctx, t, "FAIL_service_logs_not_found")
		t.Fatalf("❌ 'SERVICE LOGS' section not found in UI")
	}
	t.Log("✓ 'SERVICE LOGS' section found")

	// Final success screenshot
	takeScreenshot(ctx, t, "SUCCESS_all_checks_passed")
	t.Log("✅ All UI checks passed successfully")
}
