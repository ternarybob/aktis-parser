// -----------------------------------------------------------------------
// Last Modified: Thursday, 2nd October 2025 12:27:12 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/bobmc/aktis-parser/internal/common"
	bolt "go.etcd.io/bbolt"
)

type AuthData struct {
	Cookies   []*http.Cookie         `json:"cookies"`
	Tokens    map[string]interface{} `json:"tokens"`
	UserAgent string                 `json:"userAgent"`
	BaseURL   string                 `json:"baseUrl"`
	Timestamp int64                  `json:"timestamp"`
}

type Scraper struct {
	client    *http.Client
	baseURL   string
	userAgent string
	cloudId   string
	atlToken  string
	db        *bolt.DB
}

func NewScraper(dbPath string) (*Scraper, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Create buckets
	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("projects"))
		tx.CreateBucketIfNotExists([]byte("issues"))
		tx.CreateBucketIfNotExists([]byte("confluence_pages"))
		return nil
	})

	return &Scraper{db: db}, nil
}

func (s *Scraper) UpdateAuth(authData *AuthData) error {
	// Setup HTTP client with cookies
	jar, _ := cookiejar.New(nil)
	s.client = &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// Set cookies
	baseURL, _ := url.Parse(authData.BaseURL)
	s.client.Jar.SetCookies(baseURL, authData.Cookies)

	// Store metadata
	s.baseURL = authData.BaseURL
	s.userAgent = authData.UserAgent

	// Extract tokens
	if cloudId, ok := authData.Tokens["cloudId"].(string); ok {
		s.cloudId = cloudId
	}
	if atlToken, ok := authData.Tokens["atlToken"].(string); ok {
		s.atlToken = atlToken
	}

	log.Printf("Auth updated: %s", s.baseURL)
	return nil
}

func (s *Scraper) makeRequest(method, path string) ([]byte, error) {
	url := s.baseURL + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers to mimic browser
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

// Scrape all projects
func (s *Scraper) ScrapeProjects() error {
	log.Println("Scraping projects...")

	// Use Jira REST API
	data, err := s.makeRequest("GET", "/rest/api/3/project")
	if err != nil {
		return err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("failed to parse projects: %w", err)
	}

	// Store each project
	s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("projects"))
		for _, project := range projects {
			key := project["key"].(string)
			value, _ := json.Marshal(project)
			bucket.Put([]byte(key), value)
			log.Printf("  ✓ Stored project: %s", key)
		}
		return nil
	})

	// Now scrape issues for each project
	for _, project := range projects {
		s.ScrapeProjectIssues(project["key"].(string))
	}

	return nil
}

// Scrape all issues for a project
func (s *Scraper) ScrapeProjectIssues(projectKey string) error {
	log.Printf("Scraping issues for project %s...", projectKey)

	startAt := 0
	maxResults := 50

	for {
		// Paginate through issues
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

		// Store issues
		s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("issues"))
			for _, issue := range result.Issues {
				key := issue["key"].(string)
				value, _ := json.Marshal(issue)
				bucket.Put([]byte(key), value)
			}
			return nil
		})

		log.Printf("  ✓ Stored %d issues (total: %d)", len(result.Issues), result.Total)

		startAt += maxResults
		if startAt >= result.Total {
			break
		}

		time.Sleep(500 * time.Millisecond) // Rate limiting
	}

	return nil
}

// Scrape Confluence spaces
func (s *Scraper) ScrapeConfluence() error {
	log.Println("Scraping Confluence spaces...")

	// Get all spaces
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

	// For each space, get pages
	for _, space := range spaces.Results {
		spaceKey := space["key"].(string)
		s.ScrapeSpacePages(spaceKey)
	}

	return nil
}

func (s *Scraper) ScrapeSpacePages(spaceKey string) error {
	log.Printf("Scraping pages for space %s...", spaceKey)

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

		// Store pages
		s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("confluence_pages"))
			for _, page := range result.Results {
				id := page["id"].(string)
				value, _ := json.Marshal(page)
				bucket.Put([]byte(id), value)
			}
			return nil
		})

		log.Printf("  ✓ Stored %d pages", len(result.Results))

		start += limit
		if len(result.Results) < limit {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// Full scrape orchestration
func (s *Scraper) ScrapeAll() error {
	log.Println("=== Starting full scrape ===")

	if err := s.ScrapeProjects(); err != nil {
		return fmt.Errorf("project scrape failed: %v", err)
	}

	if err := s.ScrapeConfluence(); err != nil {
		log.Printf("Confluence scrape failed: %v", err)
		// Continue anyway
	}

	log.Println("=== Scrape complete ===")
	return nil
}

// HTTP handlers
func handleAuthUpdate(scraper *Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var authData AuthData
		if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := scraper.UpdateAuth(&authData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Start scraping in background
		go func() {
			if err := scraper.ScrapeAll(); err != nil {
				log.Printf("Scrape error: %v", err)
			}
		}()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "authenticated",
			"message": "Scraping started",
		})
	}
}

func main() {
	// 1. Load configuration
	config, err := common.LoadConfig("")
	if err != nil {
		log.Printf("Warning: Failed to load config: %v (using defaults)", err)
		config = common.DefaultConfig()
	}

	// 2. Initialize logging with arbor
	if err := common.InitLogger(&config.Logging); err != nil {
		log.Printf("Warning: Failed to initialize logger: %v", err)
	}
	logger := common.GetLogger()

	// 3. Print startup banner
	logFilePath := common.GetLogFilePath()
	common.PrintBanner(
		config.Parser.Name,
		config.Parser.Environment,
		"extension-auth", // mode
		logFilePath,
	)

	// Initialize scraper
	scraper, err := NewScraper(config.Storage.DatabasePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize scraper")
	}
	defer scraper.db.Close()

	// HTTP server
	http.HandleFunc("/api/auth", handleAuthUpdate(scraper))

	// Optional: manual trigger
	http.HandleFunc("/api/scrape", func(w http.ResponseWriter, r *http.Request) {
		go scraper.ScrapeAll()
		w.WriteHeader(http.StatusOK)
	})

	// Version endpoint
	http.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version":    common.GetVersion(),
			"build":      common.GetBuild(),
			"git_commit": common.GetGitCommit(),
		})
	})

	// Health check endpoint
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Add 404 catch-all handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only handle paths that haven't been matched
		if r.URL.Path != "/" && !isAPIPath(r.URL.Path) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Not Found",
				"path":    r.URL.Path,
				"message": "The requested endpoint does not exist",
			})
			return
		}

		// Root path - return API info
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service": "Aktis Parser",
				"version": common.GetVersion(),
				"status":  "running",
			})
			return
		}
	})

	// Start server
	addr := fmt.Sprintf(":%d", config.Parser.Port)
	logger.Info().Str("address", addr).Msg("Service starting")
	logger.Info().Msg("Install Chrome extension and click icon when logged into Jira/Confluence")

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}

// Helper to check if path is a registered API path
func isAPIPath(path string) bool {
	registeredPaths := []string{
		"/api/auth",
		"/api/scrape",
		"/api/version",
		"/api/health",
	}
	for _, p := range registeredPaths {
		if path == p {
			return true
		}
	}
	return false
}
