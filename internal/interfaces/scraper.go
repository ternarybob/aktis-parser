package interfaces

import "net/http"

// Scraper defines the interface for all scraper implementations
type Scraper interface {
	// UpdateAuth updates the scraper's authentication state
	UpdateAuth(authData *AuthData) error

	// ScrapeAll performs a full scrape of all data sources
	ScrapeAll() error

	// Close closes the scraper and releases resources
	Close() error
}

// AuthData represents authentication data from browser extension
type AuthData struct {
	Cookies   []*http.Cookie         `json:"cookies"`
	Tokens    map[string]interface{} `json:"tokens"`
	UserAgent string                 `json:"userAgent"`
	BaseURL   string                 `json:"baseUrl"`
	Timestamp int64                  `json:"timestamp"`
}
