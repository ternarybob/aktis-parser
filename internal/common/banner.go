package common

import (
	"fmt"
	"strings"

	"github.com/ternarybob/banner"
)

// PrintBanner displays the application startup banner
func PrintBanner(serviceName, environment, mode, logFile string) {
	version := GetVersion()
	build := GetBuild()

	// Create banner with custom styling - GREEN for parser
	b := banner.New().
		SetStyle(banner.StyleDouble).
		SetBorderColor(banner.ColorGreen).
		SetTextColor(banner.ColorWhite).
		SetBold(true).
		SetWidth(80)

	fmt.Printf("\n")

	// Print banner header
	b.PrintTopLine()
	b.PrintCenteredText("AKTIS PARSER")
	b.PrintCenteredText("Web Scraping Service for Jira/Confluence")
	b.PrintSeparatorLine()

	// Print version and runtime information
	b.PrintKeyValue("Version", version, 15)
	b.PrintKeyValue("Build", build, 15)
	b.PrintKeyValue("Environment", environment, 15)
	b.PrintKeyValue("Mode", mode, 15)
	b.PrintBottomLine()

	fmt.Printf("\n")

	// Print configuration details
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   • Config File: aktis-parser.toml\n")

	// Show log file if provided
	if logFile != "" {
		pattern := strings.Replace(logFile, ".log", ".{YYYY-MM-DDTHH-MM-SS}.log", 1)
		fmt.Printf("   • Log File: %s\n", pattern)
	}
	fmt.Printf("\n")

	// Print parser information
	_printParserInfo()
	fmt.Printf("\n")
}

// _printParserInfo displays the parser capabilities
func _printParserInfo() {
	fmt.Printf("🎯 Parser Capabilities:\n")
	fmt.Printf("   • Extension-based authentication (OAuth/SSO compatible)\n")
	fmt.Printf("   • Jira project and issue scraping\n")
	fmt.Printf("   • Confluence space and page scraping\n")
	fmt.Printf("   • Local BoltDB storage\n")
	fmt.Printf("   • Rate-limited API requests\n")
}

// PrintShutdownBanner displays the application shutdown banner
func PrintShutdownBanner(serviceName string) {
	b := banner.New().
		SetStyle(banner.StyleDouble).
		SetBorderColor(banner.ColorGreen).
		SetTextColor(banner.ColorWhite).
		SetBold(true).
		SetWidth(42)

	b.PrintTopLine()
	b.PrintCenteredText("SHUTTING DOWN")
	b.PrintCenteredText(serviceName)
	b.PrintBottomLine()
	fmt.Println()
}

// PrintColorizedMessage prints a message with specified color
func PrintColorizedMessage(color, message string) {
	fmt.Printf("%s%s%s\n", color, message, banner.ColorReset)
}

// PrintSuccess prints a success message in green
func PrintSuccess(message string) {
	PrintColorizedMessage(banner.ColorGreen, fmt.Sprintf("✓ %s", message))
}

// PrintError prints an error message in red
func PrintError(message string) {
	PrintColorizedMessage(banner.ColorRed, fmt.Sprintf("✗ %s", message))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(message string) {
	PrintColorizedMessage(banner.ColorYellow, fmt.Sprintf("⚠ %s", message))
}

// PrintInfo prints an info message in cyan
func PrintInfo(message string) {
	PrintColorizedMessage(banner.ColorCyan, fmt.Sprintf("ℹ %s", message))
}
