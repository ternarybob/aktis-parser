package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bobmc/aktis-parser/internal/common"
	"github.com/ternarybob/arbor"
)

type UIHandler struct {
	logger    arbor.ILogger
	staticDir string
}

func NewUIHandler() *UIHandler {
	return &UIHandler{
		logger:    common.GetLogger(),
		staticDir: getStaticDir(),
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

// FaviconHandler serves the favicon
func (h *UIHandler) FaviconHandler(w http.ResponseWriter, r *http.Request) {
	faviconPath := filepath.Join(h.staticDir, "favicon.ico")
	if _, err := os.Stat(faviconPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, faviconPath)
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

// ParserStatusHandler returns HTML for parser status
func (h *UIHandler) ParserStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	html := `
		<table class="status-table">
			<tr>
				<td class="status-label">Projects Scraped</td>
				<td class="status-value">0</td>
			</tr>
			<tr>
				<td class="status-label">Issues Scraped</td>
				<td class="status-value">0</td>
			</tr>
			<tr>
				<td class="status-label">Confluence Pages</td>
				<td class="status-value">0</td>
			</tr>
			<tr>
				<td class="status-label">Last Scrape</td>
				<td class="status-value">Never</td>
			</tr>
		</table>
	`

	fmt.Fprint(w, html)
}
