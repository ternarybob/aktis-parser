// -----------------------------------------------------------------------
// Last Modified: Thursday, 2nd October 2025 12:27:12 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package main

import (
	"fmt"
	"net/http"

	"aktis-parser/internal/common"
	"aktis-parser/internal/handlers"
	"aktis-parser/internal/services"
)

func main() {
	// 1. Load configuration
	config, err := common.LoadConfig("")
	if err != nil {
		fmt.Printf("Warning: Failed to load config: %v (using defaults)\n", err)
		config = common.DefaultConfig()
	}

	// 2. Initialize logging with arbor
	if err := common.InitLogger(&config.Logging); err != nil {
		fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
	}
	logger := common.GetLogger()

	// 3. Print startup banner
	logFilePath := common.GetLogFilePath()
	serviceURL := fmt.Sprintf("http://localhost:%d", config.Parser.Port)
	common.PrintBanner(
		config.Parser.Name,
		config.Parser.Environment,
		"extension-auth",
		logFilePath,
		serviceURL,
	)

	// 4. Initialize Jira service
	jiraService, err := services.NewJiraScraper(config.Storage.DatabasePath, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Jira service")
	}
	defer jiraService.Close()

	// 5. Initialize handlers
	apiHandler := handlers.NewAPIHandler()
	uiHandler := handlers.NewUIHandler()
	wsHandler := handlers.NewWebSocketHandler()
	scraperHandler := handlers.NewScraperHandler(jiraService, wsHandler)
	dataHandler := handlers.NewDataHandler(jiraService)

	// Set UI logger for service
	jiraService.SetUILogger(wsHandler)

	// Set auth loader for WebSocket handler (so it can send auth on connect)
	wsHandler.SetAuthLoader(jiraService)

	// Load stored authentication if available (just to log status)
	if _, err := jiraService.LoadAuth(); err == nil {
		logger.Info().Msg("Loaded stored authentication from database")
	} else {
		logger.Debug().Err(err).Msg("No stored authentication found")
	}

	// Start WebSocket status broadcaster and log streamer
	wsHandler.StartStatusBroadcaster()
	wsHandler.StartLogStreamer()

	// 6. Register routes
	// UI routes
	http.HandleFunc("/", uiHandler.IndexHandler)
	http.HandleFunc("/jira", uiHandler.JiraPageHandler)
	http.HandleFunc("/confluence", uiHandler.ConfluencePageHandler)
	http.HandleFunc("/static/common.css", uiHandler.StaticFileHandler)
	http.HandleFunc("/favicon.ico", uiHandler.StaticFileHandler)
	http.HandleFunc("/status", uiHandler.StatusHandler)
	http.HandleFunc("/parser-status", uiHandler.ParserStatusHandler)

	// WebSocket route
	http.HandleFunc("/ws", wsHandler.HandleWebSocket)

	// API routes
	http.HandleFunc("/api/auth", scraperHandler.AuthUpdateHandler)
	http.HandleFunc("/api/scrape", scraperHandler.ScrapeHandler)
	http.HandleFunc("/api/scrape/projects", scraperHandler.ScrapeProjectsHandler)
	http.HandleFunc("/api/scrape/spaces", scraperHandler.ScrapeSpacesHandler)
	http.HandleFunc("/api/projects/refresh-cache", scraperHandler.RefreshProjectsCacheHandler)
	http.HandleFunc("/api/projects/get-issues", scraperHandler.GetProjectIssuesHandler)
	http.HandleFunc("/api/spaces/refresh-cache", scraperHandler.RefreshSpacesCacheHandler)
	http.HandleFunc("/api/spaces/get-pages", scraperHandler.GetSpacePagesHandler)
	http.HandleFunc("/api/data/clear-all", scraperHandler.ClearAllDataHandler)
	http.HandleFunc("/api/data/jira", dataHandler.GetJiraDataHandler)
	http.HandleFunc("/api/data/jira/issues", dataHandler.GetJiraIssuesHandler)
	http.HandleFunc("/api/data/confluence", dataHandler.GetConfluenceDataHandler)
	http.HandleFunc("/api/data/confluence/pages", dataHandler.GetConfluencePagesHandler)
	http.HandleFunc("/api/version", apiHandler.VersionHandler)
	http.HandleFunc("/api/health", apiHandler.HealthHandler)

	// 404 handler for unmatched API routes
	http.HandleFunc("/api/", apiHandler.NotFoundHandler)

	// 7. Start server
	addr := fmt.Sprintf(":%d", config.Parser.Port)
	logger.Info().Str("address", addr).Msg("Service starting")
	logger.Info().Msg("Install Chrome extension and click icon when logged into Jira/Confluence")
	logger.Info().Str("url", fmt.Sprintf("http://localhost%s", addr)).Msg("Web UI available")

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}
