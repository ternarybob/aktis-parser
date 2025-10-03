package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"aktis-parser/internal/interfaces"
	. "github.com/ternarybob/arbor"
	bolt "go.etcd.io/bbolt"
)

// ConfluenceScraperService implements the ConfluenceScraper interface
type ConfluenceScraperService struct {
	authService interfaces.AuthService
	db          *bolt.DB
	log         ILogger
	uiLog       UILogger
}

// NewConfluenceScraper creates a new Confluence scraper instance
func NewConfluenceScraper(db *bolt.DB, authService interfaces.AuthService, logger ILogger) (*ConfluenceScraperService, error) {
	// Create buckets
	err := db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("confluence_spaces"))
		tx.CreateBucketIfNotExists([]byte("confluence_pages"))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ConfluenceScraperService{
		db:          db,
		authService: authService,
		log:         logger,
	}, nil
}

// NewConfluenceScraperWithDB creates a new Confluence scraper instance with an existing database connection
func NewConfluenceScraperWithDB(db *bolt.DB, authService interfaces.AuthService, logger ILogger) (*ConfluenceScraperService, error) {
	// Create buckets
	err := db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("confluence_spaces"))
		tx.CreateBucketIfNotExists([]byte("confluence_pages"))
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ConfluenceScraperService{
		db:          db,
		authService: authService,
		log:         logger,
	}, nil
}

// SetUILogger sets the UI logger for broadcasting to WebSocket clients
func (s *ConfluenceScraperService) SetUILogger(uiLog UILogger) {
	s.uiLog = uiLog
}

// Close closes the scraper and releases database resources
func (s *ConfluenceScraperService) Close() error {
	return s.db.Close()
}

// makeRequest makes an authenticated HTTP request
func (s *ConfluenceScraperService) makeRequest(method, path string) ([]byte, error) {
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

// GetSpacePageCount returns the total count of pages for a Confluence space
func (s *ConfluenceScraperService) GetSpacePageCount(spaceKey string) (int, error) {
	path := fmt.Sprintf("/wiki/rest/api/content?spaceKey=%s&limit=0", spaceKey)

	s.log.Debug().
		Str("spaceKey", spaceKey).
		Str("path", path).
		Msg("Fetching page count")

	data, err := s.makeRequest("GET", path)
	if err != nil {
		s.log.Error().
			Str("spaceKey", spaceKey).
			Err(err).
			Msg("Failed to fetch page count from API")
		return -1, err
	}

	var result struct {
		Size  int `json:"size"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		s.log.Error().
			Str("spaceKey", spaceKey).
			Err(err).
			Str("data", string(data)).
			Msg("Failed to parse page count response")
		return -1, fmt.Errorf("failed to parse response: %w", err)
	}

	// Use 'total' field which contains the total number of pages in the space
	// The 'size' field only contains the number of results in the current response
	s.log.Debug().
		Str("spaceKey", spaceKey).
		Int("total", result.Total).
		Msg("Retrieved page count from API")

	return result.Total, nil
}

// ScrapeConfluence scrapes all Confluence spaces and page counts
func (s *ConfluenceScraperService) ScrapeConfluence() error {
	s.log.Info().Msg("Scraping Confluence spaces...")

	allSpaces := []map[string]interface{}{}
	start := 0
	limit := 25

	// Paginate through all spaces
	for {
		path := fmt.Sprintf("/wiki/rest/api/space?start=%d&limit=%d", start, limit)
		data, err := s.makeRequest("GET", path)
		if err != nil {
			return err
		}

		var spaces struct {
			Results []map[string]interface{} `json:"results"`
			Size    int                      `json:"size"`
		}
		if err := json.Unmarshal(data, &spaces); err != nil {
			return fmt.Errorf("failed to parse spaces: %w", err)
		}

		if len(spaces.Results) == 0 {
			break
		}

		allSpaces = append(allSpaces, spaces.Results...)
		s.log.Info().Int("count", len(spaces.Results)).Msgf("Fetched %d spaces (total so far: %d)", len(spaces.Results), len(allSpaces))

		start += limit
		if len(spaces.Results) < limit {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	s.log.Info().Int("total", len(allSpaces)).Msg("Fetched all Confluence spaces, getting page counts...")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Found %d spaces, counting pages...", len(allSpaces)))
	}

	// Get page counts for all spaces in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range allSpaces {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			mu.Lock()
			spaceKey, ok := allSpaces[index]["key"].(string)
			mu.Unlock()

			if !ok {
				return
			}

			pageCount, err := s.GetSpacePageCount(spaceKey)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.log.Warn().Str("space", spaceKey).Err(err).Msg("Failed to get page count")
				allSpaces[index]["pageCount"] = -1
			} else {
				allSpaces[index]["pageCount"] = pageCount
				s.log.Info().Str("space", spaceKey).Int("pages", pageCount).Msg("Got page count")
			}

			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	s.log.Info().Msg("Completed counting pages for all spaces")

	// Store all spaces in database with page counts
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("confluence_spaces"))
		for _, space := range allSpaces {
			spaceKey, ok := space["key"].(string)
			if !ok {
				continue
			}
			value, err := json.Marshal(space)
			if err != nil {
				continue
			}
			if err := bucket.Put([]byte(spaceKey), value); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to store spaces: %w", err)
	}

	s.log.Info().Int("total", len(allSpaces)).Msg("Stored all Confluence spaces")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Stored %d Confluence spaces - ready for selection", len(allSpaces)))
	}

	return nil
}

// GetSpacePages fetches pages for a specific Confluence space (public method for API)
func (s *ConfluenceScraperService) GetSpacePages(spaceKey string) error {
	return s.scrapeSpacePages(spaceKey)
}

// scrapeSpacePages scrapes all pages in a Confluence space using concurrent batch fetching
func (s *ConfluenceScraperService) scrapeSpacePages(spaceKey string) error {
	s.log.Info().Str("spaceKey", spaceKey).Msg("Starting to fetch Confluence pages from space")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", fmt.Sprintf("Fetching pages from space: %s", spaceKey))
	}

	// Get total page count first (note: Confluence API page count is unreliable, so we fetch anyway)
	pageCount, err := s.GetSpacePageCount(spaceKey)
	if err != nil {
		s.log.Warn().Err(err).Str("spaceKey", spaceKey).Msg("Could not get page count, will fetch until empty")
		pageCount = -1
	} else {
		s.log.Info().Str("spaceKey", spaceKey).Int("pageCount", pageCount).Msg("API reported page count (may be inaccurate)")
	}

	// Always attempt to fetch pages regardless of reported count
	// The pagination loop will naturally stop when no pages are returned

	limit := 25
	batchSize := 5 // Number of concurrent requests
	totalPages := 0
	start := 0

	for {
		// Create batch of goroutines to fetch pages concurrently
		var wg sync.WaitGroup
		var mu sync.Mutex
		batchResults := make([]struct {
			start   int
			pages   []map[string]interface{}
			err     error
			hasMore bool
		}, batchSize)

		actualBatchSize := batchSize
		// Only use pageCount for optimization if we have a valid count (> 0)
		// If pageCount is 0 or -1 (unknown), fetch until we get empty results
		if pageCount > 0 {
			remaining := pageCount - totalPages
			if remaining <= 0 {
				break
			}
			if remaining < batchSize*limit {
				actualBatchSize = (remaining + limit - 1) / limit
				if actualBatchSize == 0 {
					actualBatchSize = 1
				}
			}
		}
		// If pageCount is 0 or negative, actualBatchSize remains at batchSize
		// and we'll fetch until we get no results

		for i := 0; i < actualBatchSize; i++ {
			wg.Add(1)
			batchStart := start + (i * limit)

			go func(index int, batchStart int) {
				defer wg.Done()

				path := fmt.Sprintf("/wiki/rest/api/content?spaceKey=%s&start=%d&limit=%d&expand=body.storage,space",
					spaceKey, batchStart, limit)

				s.log.Debug().Str("path", path).Int("batch", index).Msg("Requesting pages batch")
				data, err := s.makeRequest("GET", path)
				if err != nil {
					mu.Lock()
					batchResults[index].err = err
					mu.Unlock()
					return
				}

				var result struct {
					Results []map[string]interface{} `json:"results"`
					Size    int                      `json:"size"`
				}
				if err := json.Unmarshal(data, &result); err != nil {
					mu.Lock()
					batchResults[index].err = fmt.Errorf("failed to parse pages: %w", err)
					mu.Unlock()
					return
				}

				mu.Lock()
				batchResults[index].start = batchStart
				batchResults[index].pages = result.Results
				batchResults[index].hasMore = len(result.Results) >= limit
				mu.Unlock()

				time.Sleep(100 * time.Millisecond)
			}(i, batchStart)
		}

		wg.Wait()

		// Process results in order and store in database
		foundEmpty := false
		for i := 0; i < actualBatchSize; i++ {
			if batchResults[i].err != nil {
				s.log.Error().Err(batchResults[i].err).Int("batch", i).Msg("Batch fetch error")
				if s.uiLog != nil {
					s.uiLog.BroadcastUILog("error", fmt.Sprintf("Error fetching pages: %v", batchResults[i].err))
				}
				return batchResults[i].err
			}

			if len(batchResults[i].pages) == 0 {
				foundEmpty = true
				break
			}

			// Store pages
			err = s.db.Update(func(tx *bolt.Tx) error {
				bucket := tx.Bucket([]byte("confluence_pages"))
				for _, page := range batchResults[i].pages {
					id, ok := page["id"].(string)
					if !ok {
						continue
					}
					value, err := json.Marshal(page)
					if err != nil {
						continue
					}
					if err := bucket.Put([]byte(id), value); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			totalPages += len(batchResults[i].pages)

			if s.uiLog != nil {
				progress := ""
				if pageCount > 0 {
					progress = fmt.Sprintf(" (%d/%d)", totalPages, pageCount)
				}
				s.uiLog.BroadcastUILog("info", fmt.Sprintf("Fetched %d pages from %s%s", totalPages, spaceKey, progress))
			}

			// Check if we got fewer pages than requested (end of results)
			if len(batchResults[i].pages) < limit {
				foundEmpty = true
				break
			}
		}

		if foundEmpty {
			break
		}

		start += actualBatchSize * limit

		// If we know the page count and have fetched all pages
		if pageCount > 0 && totalPages >= pageCount {
			break
		}
	}

	s.log.Info().Str("spaceKey", spaceKey).Int("totalPages", totalPages).Msg("Completed page scraping for space")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("success", fmt.Sprintf("Completed: %d pages from %s", totalPages, spaceKey))
	}

	// Update the space's pageCount in database with actual count
	err = s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("confluence_spaces"))
		if bucket == nil {
			return nil
		}

		spaceData := bucket.Get([]byte(spaceKey))
		if spaceData == nil {
			return nil
		}

		var space map[string]interface{}
		if err := json.Unmarshal(spaceData, &space); err != nil {
			return err
		}

		space["pageCount"] = totalPages
		updatedData, err := json.Marshal(space)
		if err != nil {
			return err
		}

		return bucket.Put([]byte(spaceKey), updatedData)
	})

	if err != nil {
		s.log.Warn().Err(err).Str("spaceKey", spaceKey).Msg("Failed to update space page count")
	} else {
		s.log.Info().Str("spaceKey", spaceKey).Int("pageCount", totalPages).Msg("Updated space with actual page count")
	}

	return nil
}

// GetConfluenceData returns all Confluence data (spaces and pages)
func (s *ConfluenceScraperService) GetConfluenceData() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"spaces": make([]map[string]interface{}, 0),
		"pages":  make([]map[string]interface{}, 0),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		// Get all spaces
		spaceBucket := tx.Bucket([]byte("confluence_spaces"))
		if spaceBucket != nil {
			spaceBucket.ForEach(func(k, v []byte) error {
				var space map[string]interface{}
				if err := json.Unmarshal(v, &space); err == nil {
					result["spaces"] = append(result["spaces"].([]map[string]interface{}), space)
				}
				return nil
			})
		}

		// Get all pages
		pageBucket := tx.Bucket([]byte("confluence_pages"))
		if pageBucket != nil {
			pageBucket.ForEach(func(k, v []byte) error {
				var page map[string]interface{}
				if err := json.Unmarshal(v, &page); err == nil {
					result["pages"] = append(result["pages"].([]map[string]interface{}), page)
				}
				return nil
			})
		}

		return nil
	})

	return result, err
}

// ClearSpacesCache deletes all Confluence spaces from the database
func (s *ConfluenceScraperService) ClearSpacesCache() error {
	s.log.Info().Msg("Clearing Confluence spaces cache...")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", "Clearing Confluence spaces cache...")
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		// Delete the confluence_spaces bucket
		if err := tx.DeleteBucket([]byte("confluence_spaces")); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		// Recreate the bucket
		_, err := tx.CreateBucket([]byte("confluence_spaces"))
		return err
	})

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to clear Confluence spaces cache")
		return err
	}

	s.log.Info().Msg("Confluence spaces cache cleared successfully")
	if s.uiLog != nil {
		s.uiLog.BroadcastUILog("info", "Confluence spaces cache cleared successfully")
	}
	return nil
}

// ClearAllData deletes all Confluence data from all buckets
func (s *ConfluenceScraperService) ClearAllData() error {
	s.log.Info().Msg("Clearing all Confluence data from database")

	return s.db.Update(func(tx *bolt.Tx) error {
		// Delete and recreate confluence_spaces bucket
		if err := tx.DeleteBucket([]byte("confluence_spaces")); err != nil && err != bolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete confluence_spaces bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("confluence_spaces")); err != nil {
			return fmt.Errorf("failed to recreate confluence_spaces bucket: %w", err)
		}

		// Delete and recreate confluence_pages bucket
		if err := tx.DeleteBucket([]byte("confluence_pages")); err != nil && err != bolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete confluence_pages bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("confluence_pages")); err != nil {
			return fmt.Errorf("failed to recreate confluence_pages bucket: %w", err)
		}

		s.log.Info().Msg("All Confluence data cleared successfully")
		return nil
	})
}

// GetSpaceCount returns the count of Confluence spaces in the database
func (s *ConfluenceScraperService) GetSpaceCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("confluence_spaces"))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}

// GetPageCount returns the count of Confluence pages in the database
func (s *ConfluenceScraperService) GetPageCount() int {
	count := 0
	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("confluence_pages"))
		if bucket != nil {
			count = bucket.Stats().KeyN
		}
		return nil
	})
	return count
}
