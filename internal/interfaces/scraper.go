package interfaces

import (
	"net/http"
	"time"
)

// AuthService manages authentication state and HTTP client configuration
type AuthService interface {
	// UpdateAuth updates authentication state and configures HTTP client
	UpdateAuth(authData *AuthData) error

	// IsAuthenticated checks if valid authentication exists
	IsAuthenticated() bool

	// LoadAuth loads authentication from storage
	LoadAuth() (*AuthData, error)

	// GetHTTPClient returns configured HTTP client with cookies
	GetHTTPClient() *http.Client

	// GetBaseURL returns the base URL for API requests
	GetBaseURL() string

	// GetUserAgent returns the user agent string
	GetUserAgent() string

	// GetCloudID returns the Atlassian cloud ID
	GetCloudID() string

	// GetAtlToken returns the atl_token for CSRF protection
	GetAtlToken() string
}

// BaseScraper defines common methods for all scraper implementations
type BaseScraper interface {
	// Close closes the scraper and releases resources
	Close() error
}

// Scraper is a unified interface for backward compatibility with handlers
// Handlers use type assertions to access specific methods from JiraScraper or ConfluenceScraper
type Scraper interface {
	BaseScraper
	ScrapeAll() error
	ScrapeProjects() error
}

// JiraScraper defines the interface for Jira scraping operations
type JiraScraper interface {
	BaseScraper

	// ScrapeProjects scrapes Jira projects with issue counts
	ScrapeProjects() error

	// GetProjectIssues retrieves all issues for a given project
	GetProjectIssues(projectKey string) error

	// GetProjectIssueCount returns the total count of issues for a project
	GetProjectIssueCount(projectKey string) (int, error)

	// DeleteProjectIssues deletes all issues for a given project
	DeleteProjectIssues(projectKey string) error

	// ClearProjectsCache deletes all projects from the database
	ClearProjectsCache() error

	// GetJiraData returns all Jira data (projects and issues)
	GetJiraData() (map[string]interface{}, error)
}

// ConfluenceScraper defines the interface for Confluence scraping operations
type ConfluenceScraper interface {
	BaseScraper

	// ScrapeConfluence scrapes Confluence spaces with page counts
	ScrapeConfluence() error

	// GetSpacePages fetches pages for a specific Confluence space
	GetSpacePages(spaceKey string) error

	// GetSpacePageCount returns the total count of pages for a space
	GetSpacePageCount(spaceKey string) (int, error)

	// ClearSpacesCache deletes all Confluence spaces from the database
	ClearSpacesCache() error

	// GetConfluenceData returns all Confluence data (spaces and pages)
	GetConfluenceData() (map[string]interface{}, error)
}

// ClearableData defines interface for services that can clear their data
type ClearableData interface {
	ClearAllData() error
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

// LoggingService interface defines methods for application logging
type LoggingService interface {
	// Core logging methods
	Debug(message string)
	Info(message string)
	Warn(message string)
	Error(message string, err error)

	// Structured logging with fields
	WithField(key string, value interface{}) LogEntry
	WithFields(fields map[string]interface{}) LogEntry

	// UI broadcast
	BroadcastToUI(level, message string)
}

// LogEntry represents a chainable log entry
type LogEntry interface {
	Debug(message string)
	Info(message string)
	Warn(message string)
	Error(message string, err error)
	Msg(message string)
}
