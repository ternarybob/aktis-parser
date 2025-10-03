package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"aktis-parser/internal/interfaces"
	. "github.com/ternarybob/arbor"
	bolt "go.etcd.io/bbolt"
)

// JiraScraper implements the Scraper interface for Atlassian Jira
type JiraScraper struct {
	authService interfaces.AuthService
	db          *bolt.DB
	log         ILogger
	uiLog       UILogger
}

// NewJiraScraper creates a new Jira scraper instance
func NewJiraScraper(db *bolt.DB, authService interfaces.AuthService, logger ILogger) (*JiraScraper, error) {
	// Create buckets
	err := db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("projects"))
		tx.CreateBucketIfNotExists([]byte("issues"))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &JiraScraper{
		db:          db,
		authService: authService,
		log:         logger,
	}, nil
}

// SetUILogger sets the UI logger for broadcasting to WebSocket clients
func (s *JiraScraper) SetUILogger(uiLog UILogger) {
	s.uiLog = uiLog
}

// GetDB returns the database connection for sharing with other services
func (s *JiraScraper) GetDB() *bolt.DB {
	return s.db
}

// Close closes the scraper and releases database resources
func (s *JiraScraper) Close() error {
	return s.db.Close()
}

// makeRequest makes an authenticated HTTP request
func (s *JiraScraper) makeRequest(method, path string) ([]byte, error) {
	url := s.authService.GetBaseURL() + path

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.authService.GetUserAgent())
	req.Header.Set("Accept", "application/json, text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.authService.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)

	// Log all non-200 responses
	if resp.StatusCode != 200 {
		s.log.Error().
			Str("url", url).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("HTTP request failed")

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("auth expired (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if readErr != nil {
		return nil, readErr
	}

	return body, nil
}

// GetProjectIssueCount returns the total count of issues for a project
func (s *JiraScraper) GetProjectIssueCount(projectKey string) (int, error) {
	// Atlassian Cloud /rest/api/3/search/jql endpoint no longer returns a `total` field
	// We need to fetch issues with maxResults=5000 (API max) and count them
	// Using fields=-all to minimize response size since we only need the count
	jql := fmt.Sprintf("project=\"%s\"", projectKey)
	encodedJQL := url.QueryEscape(jql)
	path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&maxResults=5000&fields=-all", encodedJQL)

	s.log.Debug().
		Str("project", projectKey).
		Str("jql", jql).
		Str("path", path).
		Msg("Fetching issue count")

	data, err := s.makeRequest("GET", path)
	if err != nil {
		s.log.Error().
			Str("project", projectKey).
			Err(err).
			Msg("Failed to fetch issue count from API")
		return 0, err
	}

	var result struct {
		Issues []interface{} `json:"issues"`
		IsLast bool          `json:"isLast"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		s.log.Error().
			Str("project", projectKey).
			Err(err).
			Str("data", string(data)).
			Msg("Failed to parse issue count response")
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	count := len(result.Issues)

	s.log.Info().
		Str("project", projectKey).
		Int("count", count).
		Str("isLast", fmt.Sprintf("%v", result.IsLast)).
		Msg("Retrieved issue count")

	// If isLast is false, there are more than 5000 issues
	// For simplicity, we return the count we got (5000+)
	return count, nil
}

// ScrapeProjects scrapes all Jira projects and their issues
func (s *JiraScraper) ScrapeProjects() error {
	s.log.Info().Msg("Scraping projects...")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", "Fetching projects from Jira...")
	}

	data, err := s.makeRequest("GET", "/rest/api/3/project")
	if err != nil {
		return err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("failed to parse projects: %w", err)
	}

	s.log.Info().Msgf("Found %d projects", len(projects))
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Found %d projects, counting issues in parallel...", len(projects)))
	}

	// Get issue counts for each project in parallel using goroutines
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range projects {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			mu.Lock()
			projectKey := projects[index]["key"].(string)
			mu.Unlock()

			issueCount, err := s.GetProjectIssueCount(projectKey)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.log.Warn().Str("project", projectKey).Err(err).Msg("Failed to get issue count")
				projects[index]["issueCount"] = 0
			} else {
				projects[index]["issueCount"] = issueCount
				s.log.Info().Str("project", projectKey).Int("issues", issueCount).Msg("Got issue count")
			}

			time.Sleep(100 * time.Millisecond) // Reduced rate limiting since parallel
		}(i)
	}

	// Wait for all issue counts to complete
	wg.Wait()
	s.log.Info().Msg("Completed counting issues for all projects")

	s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("projects"))
		for _, project := range projects {
			key := project["key"].(string)
			value, _ := json.Marshal(project)
			bucket.Put([]byte(key), value)

			if s.uiLog != nil {
				projectName := "Unknown"
				if name, ok := project["name"].(string); ok {
					projectName = name
				}

				// Try to get issue count - handle both int and float64 from JSON
				issueCount := 0
				if count, ok := project["issueCount"].(int); ok {
					issueCount = count
				} else if count, ok := project["issueCount"].(float64); ok {
					issueCount = int(count)
				}

				s.log.Info().
					Str("project", key).
					Str("name", projectName).
					Int("issueCount", issueCount).
					Msg("Stored project")
				s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored project: %s (%s) - %d issues", key, projectName, issueCount))
			}
		}
		return nil
	})

	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Successfully synced %d projects", len(projects)))
	}

	return nil
}

// DeleteProjectIssues deletes all issues for a given project
func (s *JiraScraper) DeleteProjectIssues(projectKey string) error {
	s.log.Info().Str("project", projectKey).Msg("Deleting issues for project")

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("issues"))
		if bucket == nil {
			return nil
		}

		// Find and delete all issues for this project
		c := bucket.Cursor()
		var keysToDelete [][]byte

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var issue map[string]interface{}
			if err := json.Unmarshal(v, &issue); err != nil {
				continue
			}

			// Check if issue belongs to this project
			if fields, ok := issue["fields"].(map[string]interface{}); ok {
				if project, ok := fields["project"].(map[string]interface{}); ok {
					if key, ok := project["key"].(string); ok && key == projectKey {
						keysToDelete = append(keysToDelete, k)
					}
				}
			}
		}

		// Delete all matching keys
		for _, k := range keysToDelete {
			if err := bucket.Delete(k); err != nil {
				return err
			}
		}

		s.log.Info().
			Str("project", projectKey).
			Int("deleted", len(keysToDelete)).
			Msg("Deleted project issues")

		return nil
	})
}

// GetProjectIssues retrieves all issues for a given project and syncs them
func (s *JiraScraper) GetProjectIssues(projectKey string) error {
	// First delete existing issues for this project
	if err := s.DeleteProjectIssues(projectKey); err != nil {
		s.log.Error().Err(err).Str("project", projectKey).Msg("Failed to delete old issues")
		return err
	}

	// Now fetch fresh issues
	return s.scrapeProjectIssues(projectKey)
}

// scrapeProjectIssues scrapes all issues for a given project using count-based pagination
func (s *JiraScraper) scrapeProjectIssues(projectKey string) error {
	s.log.Info().Str("project", projectKey).Msg("Scraping issues for project")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Fetching issues for project: %s", projectKey))
	}

	startAt := 0
	maxResults := 100
	totalFetched := 0
	maxIterations := 200 // Safety limit: max 20,000 issues (200 * 100)
	seenIssueKeys := make(map[string]bool)

	for iteration := 0; iteration < maxIterations; iteration++ {
		// Use /rest/api/3/search/jql endpoint with properly escaped JQL
		// JQL syntax: project = "PROJECT_KEY"
		jql := fmt.Sprintf("project=\"%s\"", projectKey)
		encodedJQL := url.QueryEscape(jql)
		path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&startAt=%d&maxResults=%d&fields=key,summary,status,issuetype,project",
			encodedJQL, startAt, maxResults)

		s.log.Info().
			Str("project", projectKey).
			Str("jql", jql).
			Int("startAt", startAt).
			Int("maxResults", maxResults).
			Int("iteration", iteration+1).
			Msg("Fetching issues batch")

		data, err := s.makeRequest("GET", path)
		if err != nil {
			s.log.Error().Err(err).Str("project", projectKey).Str("path", path).Msg("Failed to fetch issues")
			if s.uiLog != nil {
				s.uiLog.BroadcastUILog("error", fmt.Sprintf("Failed to fetch issues for %s: %v", projectKey, err))
			}
			return err
		}

		var result struct {
			Issues []map[string]interface{} `json:"issues"`
			IsLast bool                     `json:"isLast"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			s.log.Error().Err(err).Str("project", projectKey).Msg("Failed to parse issues response")
			if s.uiLog != nil {
				s.uiLog.BroadcastUILog("error", fmt.Sprintf("Failed to parse issues for %s: %v", projectKey, err))
			}
			return fmt.Errorf("failed to parse issues: %w", err)
		}

		issuesInBatch := len(result.Issues)

		s.log.Info().
			Str("project", projectKey).
			Int("issuesInBatch", issuesInBatch).
			Int("startAt", startAt).
			Str("isLast", fmt.Sprintf("%v", result.IsLast)).
			Msg("Received issues batch")

		// If no issues returned, we're done
		if issuesInBatch == 0 {
			s.log.Info().
				Str("project", projectKey).
				Int("totalFetched", totalFetched).
				Msg("No more issues, stopping pagination")
			break
		}

		// Verify issues belong to the requested project and check for duplicates
		duplicateCount := 0
		newIssuesCount := 0
		wrongProjectCount := 0
		for _, issue := range result.Issues {
			issueKey := ""
			if key, ok := issue["key"].(string); ok {
				issueKey = key
			}

			// Check which project this issue belongs to
			actualProjectKey := ""
			if fields, ok := issue["fields"].(map[string]interface{}); ok {
				if project, ok := fields["project"].(map[string]interface{}); ok {
					if key, ok := project["key"].(string); ok {
						actualProjectKey = key
					}
				}
			}

			// Warn if issue belongs to different project
			if actualProjectKey != "" && actualProjectKey != projectKey {
				wrongProjectCount++
				s.log.Warn().
					Str("requestedProject", projectKey).
					Str("actualProject", actualProjectKey).
					Str("issueKey", issueKey).
					Msg("API returned issue from wrong project")
			}

			// Check for duplicates
			if issueKey != "" {
				if seenIssueKeys[issueKey] {
					duplicateCount++
				} else {
					seenIssueKeys[issueKey] = true
					newIssuesCount++
				}
			}
		}

		// Log warning if wrong project issues detected (but don't stop scraping)
		// The JQL query with quoted project key should prevent this
		if wrongProjectCount > 0 {
			s.log.Warn().
				Str("project", projectKey).
				Int("wrongProjectIssues", wrongProjectCount).
				Int("correctIssues", issuesInBatch-wrongProjectCount).
				Int("totalInBatch", issuesInBatch).
				Msg("Jira API returned some issues from wrong project - investigate JQL query")
		}

		// If all issues are duplicates, we're looping on same data
		if duplicateCount > 0 && newIssuesCount == 0 {
			s.log.Info().
				Str("project", projectKey).
				Int("duplicates", duplicateCount).
				Int("totalFetched", totalFetched).
				Msg("All issues are duplicates, stopping pagination")
			break
		}

		if duplicateCount > 0 {
			s.log.Warn().
				Str("project", projectKey).
				Int("duplicates", duplicateCount).
				Int("newIssues", newIssuesCount).
				Msg("Detected duplicate issues in batch")
		}

		// Store issues in database (only new ones)
		storedCount := 0
		if err := s.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("issues"))
			if bucket == nil {
				return fmt.Errorf("issues bucket not found")
			}
			for _, issue := range result.Issues {
				key, ok := issue["key"].(string)
				if !ok {
					s.log.Warn().Msg("Issue missing key field, skipping")
					continue
				}
				value, err := json.Marshal(issue)
				if err != nil {
					s.log.Warn().Str("key", key).Err(err).Msg("Failed to marshal issue")
					continue
				}
				if err := bucket.Put([]byte(key), value); err != nil {
					return fmt.Errorf("failed to store issue %s: %w", key, err)
				}
				storedCount++
			}
			return nil
		}); err != nil {
			s.log.Error().Err(err).Str("project", projectKey).Msg("Failed to store issues in database")
			if s.uiLog != nil {
				s.uiLog.BroadcastUILog("error", fmt.Sprintf("Failed to store issues: %v", err))
			}
			return err
		}

		totalFetched += newIssuesCount

		s.log.Info().
			Str("project", projectKey).
			Int("batchSize", issuesInBatch).
			Int("newIssues", newIssuesCount).
			Int("duplicates", duplicateCount).
			Int("totalFetched", totalFetched).
			Msg("Stored issues batch")

		if s.uiLog != nil {
			s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored %d new issues for %s (total: %d)", newIssuesCount, projectKey, totalFetched))
		}

		// Stop if isLast flag is true
		if result.IsLast {
			s.log.Info().
				Str("project", projectKey).
				Int("totalFetched", totalFetched).
				Msg("Reached last page (isLast=true)")
			break
		}

		// Stop if we got fewer issues than requested (indicates last page)
		if issuesInBatch < maxResults {
			s.log.Info().
				Str("project", projectKey).
				Int("received", issuesInBatch).
				Int("requested", maxResults).
				Int("totalFetched", totalFetched).
				Msg("Received partial batch, stopping pagination")
			break
		}

		// Increment startAt based on actual issues fetched
		startAt += issuesInBatch
		time.Sleep(300 * time.Millisecond)
	}

	s.log.Info().
		Str("project", projectKey).
		Int("totalIssues", totalFetched).
		Msg("Completed fetching issues")

	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("success", fmt.Sprintf("Completed: %d issues for %s", totalFetched, projectKey))
	}

	return nil
}

// GetJiraData returns all Jira data (projects and issues)
func (s *JiraScraper) GetJiraData() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"projects": make([]map[string]interface{}, 0),
		"issues":   make([]map[string]interface{}, 0),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		// Get all projects
		projectBucket := tx.Bucket([]byte("projects"))
		if projectBucket != nil {
			projectBucket.ForEach(func(k, v []byte) error {
				var project map[string]interface{}
				if err := json.Unmarshal(v, &project); err == nil {
					result["projects"] = append(result["projects"].([]map[string]interface{}), project)
				}
				return nil
			})
		}

		// Get all issues
		issueBucket := tx.Bucket([]byte("issues"))
		if issueBucket != nil {
			issueBucket.ForEach(func(k, v []byte) error {
				var issue map[string]interface{}
				if err := json.Unmarshal(v, &issue); err == nil {
					result["issues"] = append(result["issues"].([]map[string]interface{}), issue)
				}
				return nil
			})
		}

		return nil
	})

	return result, err
}

// ClearAllData deletes all data from all buckets (projects, issues)
func (s *JiraScraper) ClearAllData() error {
	s.log.Info().Msg("Clearing all Jira data from database")

	return s.db.Update(func(tx *bolt.Tx) error {
		// Delete and recreate projects bucket
		if err := tx.DeleteBucket([]byte("projects")); err != nil && err != bolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete projects bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("projects")); err != nil {
			return fmt.Errorf("failed to recreate projects bucket: %w", err)
		}

		// Delete and recreate issues bucket
		if err := tx.DeleteBucket([]byte("issues")); err != nil && err != bolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete issues bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("issues")); err != nil {
			return fmt.Errorf("failed to recreate issues bucket: %w", err)
		}

		s.log.Info().Msg("All Jira data cleared successfully")
		return nil
	})
}

// ClearProjectsCache deletes all projects from the database
func (s *JiraScraper) ClearProjectsCache() error {
	s.log.Info().Msg("Clearing projects cache...")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", "Clearing projects cache...")
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		// Delete the projects bucket
		if err := tx.DeleteBucket([]byte("projects")); err != nil {
			return err
		}
		// Recreate the bucket
		_, err := tx.CreateBucket([]byte("projects"))
		return err
	})

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to clear projects cache")
		return err
	}

	s.log.Info().Msg("Projects cache cleared successfully")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", "Projects cache cleared successfully")
	}
	return nil
}

// ScrapeAll performs a full scrape of Jira projects
func (s *JiraScraper) ScrapeAll() error {
	s.log.Info().Msg("=== Starting Jira scrape ===")

	if err := s.ScrapeProjects(); err != nil {
		return fmt.Errorf("project scrape failed: %v", err)
	}

	s.log.Info().Msg("=== Jira scrape complete ===")
	return nil
}

// GetProjectCount returns the count of projects in the database
func (s *JiraScraper) GetProjectCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("projects"))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}

// GetIssueCount returns the count of issues in the database
func (s *JiraScraper) GetIssueCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("issues"))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}
