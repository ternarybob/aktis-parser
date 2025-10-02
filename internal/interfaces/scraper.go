package interfaces

import (
	"net/http"
	"time"
)

// Scraper defines the interface for all scraper implementations
type Scraper interface {
	// UpdateAuth updates the scraper's authentication state
	UpdateAuth(authData *AuthData) error

	// ScrapeAll performs a full scrape of all data sources
	ScrapeAll() error

	// ScrapeProjects scrapes Jira projects and their issues
	ScrapeProjects() error

	// ScrapeConfluence scrapes Confluence spaces and pages
	ScrapeConfluence() error

	// IsAuthenticated checks if the scraper has valid authentication
	IsAuthenticated() bool

	// GetJiraData returns all Jira data (projects and issues)
	GetJiraData() (map[string]interface{}, error)

	// GetConfluenceData returns all Confluence data (pages)
	GetConfluenceData() (map[string]interface{}, error)

	// Close closes the scraper and releases resources
	Close() error
}

// ExtensionCookie represents a cookie from the browser extension
// This uses string for SameSite since JavaScript sends it as a string
type ExtensionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"` // Unix timestamp
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
	SameSite string `json:"sameSite"` // "Strict", "Lax", "None", or empty
}

// ToHTTPCookie converts ExtensionCookie to http.Cookie
func (ec *ExtensionCookie) ToHTTPCookie() *http.Cookie {
	cookie := &http.Cookie{
		Name:     ec.Name,
		Value:    ec.Value,
		Domain:   ec.Domain,
		Path:     ec.Path,
		Secure:   ec.Secure,
		HttpOnly: ec.HTTPOnly,
	}

	// Convert expires timestamp to time.Time
	if ec.Expires > 0 {
		cookie.Expires = time.Unix(ec.Expires, 0)
	}

	// Convert SameSite string to http.SameSite
	switch ec.SameSite {
	case "Strict", "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "Lax", "lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "None", "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteDefaultMode
	}

	return cookie
}

// AuthData represents authentication data from browser extension
type AuthData struct {
	Cookies   []*ExtensionCookie     `json:"cookies"`
	Tokens    map[string]interface{} `json:"tokens"`
	UserAgent string                 `json:"userAgent"`
	BaseURL   string                 `json:"baseUrl"`
	Timestamp int64                  `json:"timestamp"`
}

// GetHTTPCookies converts all extension cookies to http.Cookie format
func (ad *AuthData) GetHTTPCookies() []*http.Cookie {
	cookies := make([]*http.Cookie, len(ad.Cookies))
	for i, ec := range ad.Cookies {
		cookies[i] = ec.ToHTTPCookie()
	}
	return cookies
}
