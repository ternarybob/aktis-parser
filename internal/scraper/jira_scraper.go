package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/bobmc/aktis-parser/internal/interfaces"
	. "github.com/ternarybob/arbor"
	bolt "go.etcd.io/bbolt"
)

// UILogger interface for broadcasting to UI
type UILogger interface {
	BroadcastUILog(level, message string)
}

// JiraScraper implements the Scraper interface for Atlassian Jira/Confluence
type JiraScraper struct {
	client    *http.Client
	baseURL   string
	userAgent string
	cloudId   string
	atlToken  string
	db        *bolt.DB
	log       ILogger
	uiLog     UILogger
}

// NewJiraScraper creates a new Jira/Confluence scraper instance
func NewJiraScraper(dbPath string, logger ILogger) (*JiraScraper, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Create buckets
	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("projects"))
		tx.CreateBucketIfNotExists([]byte("issues"))
		tx.CreateBucketIfNotExists([]byte("confluence_pages"))
		tx.CreateBucketIfNotExists([]byte("auth"))
		return nil
	})

	return &JiraScraper{
		db:  db,
		log: logger,
	}, nil
}

// SetUILogger sets the UI logger for broadcasting to WebSocket clients
func (s *JiraScraper) SetUILogger(uiLog UILogger) {
	s.uiLog = uiLog
}

// Close closes the scraper and releases database resources
func (s *JiraScraper) Close() error {
	return s.db.Close()
}

// UpdateAuth updates the scraper's authentication state
func (s *JiraScraper) UpdateAuth(authData *interfaces.AuthData) error {
	jar, _ := cookiejar.New(nil)
	s.client = &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	baseURL, _ := url.Parse(authData.BaseURL)
	s.client.Jar.SetCookies(baseURL, authData.GetHTTPCookies())

	s.baseURL = authData.BaseURL
	s.userAgent = authData.UserAgent

	if cloudId, ok := authData.Tokens["cloudId"].(string); ok {
		s.cloudId = cloudId
	}
	if atlToken, ok := authData.Tokens["atlToken"].(string); ok {
		s.atlToken = atlToken
	}

	// Store authentication in database
	if err := s.storeAuth(authData); err != nil {
		s.log.Error().Err(err).Msg("Failed to store auth in database")
	}

	s.log.Info().Str("baseURL", s.baseURL).Msg("Auth updated")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Authentication updated for %s", s.baseURL))
	}
	return nil
}

// storeAuth stores authentication data in the database
func (s *JiraScraper) storeAuth(authData *interfaces.AuthData) error {
	data, err := json.Marshal(authData)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("auth"))
		return bucket.Put([]byte("current"), data)
	})
}

// LoadAuth loads authentication from the database
func (s *JiraScraper) LoadAuth() (*interfaces.AuthData, error) {
	var authData interfaces.AuthData

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("auth"))
		data := bucket.Get([]byte("current"))
		if data == nil {
			return fmt.Errorf("no authentication stored")
		}
		return json.Unmarshal(data, &authData)
	})

	if err != nil {
		return nil, err
	}

	// Reapply auth to HTTP client
	return &authData, s.UpdateAuth(&authData)
}

// makeRequest makes an authenticated HTTP request
func (s *JiraScraper) makeRequest(method, path string) ([]byte, error) {
	url := s.baseURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json, text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("auth expired (status %d)", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ScrapeProjects scrapes all Jira projects and their issues
func (s *JiraScraper) ScrapeProjects() error {
	s.log.Info().Msg("Scraping projects...")

	data, err := s.makeRequest("GET", "/rest/api/3/project")
	if err != nil {
		return err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("failed to parse projects: %w", err)
	}

	s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("projects"))
		for _, project := range projects {
			key := project["key"].(string)
			value, _ := json.Marshal(project)
			bucket.Put([]byte(key), value)
			s.log.Info().Str("project", key).Msg("Stored project")
			if s.uiLog != nil {
				projectName := "Unknown"
				if name, ok := project["name"].(string); ok {
					projectName = name
				}
				s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored project: %s (%s)", key, projectName))
			}
		}
		return nil
	})

	for _, project := range projects {
		s.scrapeProjectIssues(project["key"].(string))
	}

	return nil
}

// scrapeProjectIssues scrapes all issues for a given project
func (s *JiraScraper) scrapeProjectIssues(projectKey string) error {
	s.log.Info().Str("project", projectKey).Msg("Scraping issues for project")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Fetching issues for project: %s", projectKey))
	}

	startAt := 0
	maxResults := 50

	for {
		path := fmt.Sprintf("/rest/api/3/search?jql=project=%s&startAt=%d&maxResults=%d",
			projectKey, startAt, maxResults)

		data, err := s.makeRequest("GET", path)
		if err != nil {
			return err
		}

		var result struct {
			Issues []map[string]interface{} `json:"issues"`
			Total  int                      `json:"total"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("failed to parse issues: %w", err)
		}

		if len(result.Issues) == 0 {
			break
		}

		s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("issues"))
			for _, issue := range result.Issues {
				key := issue["key"].(string)
				value, _ := json.Marshal(issue)
				bucket.Put([]byte(key), value)
			}
			return nil
		})

		s.log.Info().Msgf("Stored issues (count=%d, total=%d)", len(result.Issues), result.Total)
		if s.uiLog != nil {
			s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored %d issues for project %s (total: %d)", len(result.Issues), projectKey, result.Total))
		}

		startAt += maxResults
		if startAt >= result.Total {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// ScrapeConfluence scrapes all Confluence spaces and pages
func (s *JiraScraper) ScrapeConfluence() error {
	s.log.Info().Msg("Scraping Confluence spaces...")

	data, err := s.makeRequest("GET", "/wiki/rest/api/space")
	if err != nil {
		return err
	}

	var spaces struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(data, &spaces); err != nil {
		return fmt.Errorf("failed to parse spaces: %w", err)
	}

	for _, space := range spaces.Results {
		spaceKey := space["key"].(string)
		s.scrapeSpacePages(spaceKey)
	}

	return nil
}

// scrapeSpacePages scrapes all pages in a Confluence space
func (s *JiraScraper) scrapeSpacePages(spaceKey string) error {
	s.log.Info().Str("workspace", spaceKey).Msg("Fetching Confluence pages from workspace")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Fetching pages from workspace: %s", spaceKey))
	}

	start := 0
	limit := 25

	for {
		path := fmt.Sprintf("/wiki/rest/api/content?spaceKey=%s&start=%d&limit=%d&expand=body.storage",
			spaceKey, start, limit)

		data, err := s.makeRequest("GET", path)
		if err != nil {
			return err
		}

		var result struct {
			Results []map[string]interface{} `json:"results"`
			Size    int                      `json:"size"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("failed to parse pages: %w", err)
		}

		if len(result.Results) == 0 {
			break
		}

		s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("confluence_pages"))
			for _, page := range result.Results {
				id := page["id"].(string)
				value, _ := json.Marshal(page)
				bucket.Put([]byte(id), value)
			}
			return nil
		})

		s.log.Info().Msgf("Stored pages (count=%d)", len(result.Results))
		if s.uiLog != nil {
			s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored %d pages from workspace %s", len(result.Results), spaceKey))
		}

		start += limit
		if len(result.Results) < limit {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// IsAuthenticated checks if the scraper has valid authentication
func (s *JiraScraper) IsAuthenticated() bool {
	return s.client != nil && s.baseURL != ""
}

// ScrapeAll performs a full scrape of Jira and Confluence
func (s *JiraScraper) ScrapeAll() error {
	s.log.Info().Msg("=== Starting full scrape ===")

	if err := s.ScrapeProjects(); err != nil {
		return fmt.Errorf("project scrape failed: %v", err)
	}

	if err := s.ScrapeConfluence(); err != nil {
		s.log.Error().Err(err).Msg("Confluence scrape failed")
	}

	s.log.Info().Msg("=== Scrape complete ===")
	return nil
}
