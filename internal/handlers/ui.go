package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"aktis-parser/internal/common"
	"aktis-parser/internal/interfaces"
	"github.com/ternarybob/arbor"
)

type UIHandler struct {
	logger            arbor.ILogger
	staticDir         string
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
}

func NewUIHandler(jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper) *UIHandler {
	return &UIHandler{
		logger:            common.GetLogger(),
		staticDir:         getStaticDir(),
		jiraScraper:       jira,
		confluenceScraper: confluence,
	}
}

// getStaticDir finds the pages directory
func getStaticDir() string {
	dirs := []string{
		"./pages",
		"../pages",
		"../../pages",
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "."
}

// IndexHandler serves the main HTML page
func (h *UIHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	indexPath := filepath.Join(h.staticDir, "index.html")
	http.ServeFile(w, r, indexPath)
}

// StatusHandler returns HTML for service status
func (h *UIHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	html := `
		<tr>
			<td class="status-label">Parser Service</td>
			<td class="status-value status-online">ONLINE</td>
		</tr>
		<tr>
			<td class="status-label">Database</td>
			<td class="status-value status-online">CONNECTED</td>
		</tr>
		<tr>
			<td class="status-label">Extension Auth</td>
			<td class="status-value">WAITING</td>
		</tr>
	`

	fmt.Fprint(w, html)
}

// ParserStatusHandler returns HTML for parser status with database counts
func (h *UIHandler) ParserStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Get database counts directly from efficient count methods
	projectCount := h.jiraScraper.GetProjectCount()
	issueCount := h.jiraScraper.GetIssueCount()
	spaceCount := h.confluenceScraper.GetSpaceCount()
	pageCount := h.confluenceScraper.GetPageCount()

	html := fmt.Sprintf(`
		<table class="status-table">
			<tr>
				<td class="status-label">Projects Scraped</td>
				<td class="status-value">%d</td>
			</tr>
			<tr>
				<td class="status-label">Issues Scraped</td>
				<td class="status-value">%d</td>
			</tr>
			<tr>
				<td class="status-label">Confluence Spaces</td>
				<td class="status-value">%d</td>
			</tr>
			<tr>
				<td class="status-label">Confluence Pages</td>
				<td class="status-value">%d</td>
			</tr>
		</table>
	`, projectCount, issueCount, spaceCount, pageCount)

	fmt.Fprint(w, html)
}

// JiraPageHandler serves the Jira data page
func (h *UIHandler) JiraPageHandler(w http.ResponseWriter, r *http.Request) {
	jiraPath := filepath.Join(h.staticDir, "jira.html")
	http.ServeFile(w, r, jiraPath)
}

// ConfluencePageHandler serves the Confluence data page
func (h *UIHandler) ConfluencePageHandler(w http.ResponseWriter, r *http.Request) {
	confluencePath := filepath.Join(h.staticDir, "confluence.html")
	http.ServeFile(w, r, confluencePath)
}

// StaticFileHandler serves static files (CSS, favicon) from the pages/static directory
func (h *UIHandler) StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	// List of allowed static files
	allowedFiles := map[string]string{
		"/static/common.css": "static/common.css",
		"/favicon.ico":       "favicon.ico",
	}

	// Check if the requested path is allowed
	relativePath, allowed := allowedFiles[r.URL.Path]
	if !allowed {
		http.NotFound(w, r)
		return
	}

	// Construct the full path
	filePath := filepath.Join(h.staticDir, relativePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Set appropriate content type
	ext := filepath.Ext(filePath)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
}
