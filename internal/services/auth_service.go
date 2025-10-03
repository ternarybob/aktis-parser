package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"aktis-parser/internal/interfaces"
	. "github.com/ternarybob/arbor"
	bolt "go.etcd.io/bbolt"
)

// AtlassianAuthService implements the AuthService interface
type AtlassianAuthService struct {
	client    *http.Client
	baseURL   string
	userAgent string
	cloudId   string
	atlToken  string
	db        *bolt.DB
	log       ILogger
}

// NewAtlassianAuthService creates a new authentication service
func NewAtlassianAuthService(db *bolt.DB, logger ILogger) (*AtlassianAuthService, error) {
	// Create auth bucket
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("auth"))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth bucket: %w", err)
	}

	service := &AtlassianAuthService{
		db:  db,
		log: logger,
	}

	// Try to load existing auth
	if authData, err := service.LoadAuth(); err == nil {
		if updateErr := service.UpdateAuth(authData); updateErr != nil {
			logger.Warn().Err(updateErr).Msg("Failed to apply stored authentication")
		} else {
			logger.Info().Msg("Successfully loaded and applied stored authentication")
		}
	} else {
		logger.Debug().Err(err).Msg("No stored authentication found")
	}

	return service, nil
}

// UpdateAuth updates authentication state and configures HTTP client
func (s *AtlassianAuthService) UpdateAuth(authData *interfaces.AuthData) error {
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
		s.log.Debug().Str("cloudId", cloudId).Msg("CloudID extracted from auth tokens")
	} else {
		s.log.Warn().Msgf("CloudID not found in auth tokens or wrong type (tokens: %+v)", authData.Tokens)
	}

	if atlToken, ok := authData.Tokens["atlToken"].(string); ok {
		s.atlToken = atlToken
		s.log.Debug().Msg("atlToken extracted from auth tokens")
	} else {
		s.log.Warn().Msgf("atlToken not found in auth tokens or wrong type (tokens: %+v)", authData.Tokens)
	}

	// Store auth in database
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("auth"))
		authJSON, err := json.Marshal(authData)
		if err != nil {
			return err
		}
		return bucket.Put([]byte("current"), authJSON)
	})
}

// IsAuthenticated checks if valid authentication exists
func (s *AtlassianAuthService) IsAuthenticated() bool {
	// Only require HTTP client with cookies and baseURL
	// cloudId and atlToken are optional and not used in API requests
	return s.client != nil && s.baseURL != ""
}

// LoadAuth loads authentication from storage
func (s *AtlassianAuthService) LoadAuth() (*interfaces.AuthData, error) {
	var authData interfaces.AuthData
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("auth"))
		if bucket == nil {
			return fmt.Errorf("auth bucket not found")
		}
		authJSON := bucket.Get([]byte("current"))
		if authJSON == nil {
			return fmt.Errorf("no auth data found")
		}
		return json.Unmarshal(authJSON, &authData)
	})
	return &authData, err
}

// GetHTTPClient returns configured HTTP client with cookies
func (s *AtlassianAuthService) GetHTTPClient() *http.Client {
	return s.client
}

// GetBaseURL returns the base URL for API requests
func (s *AtlassianAuthService) GetBaseURL() string {
	return s.baseURL
}

// GetUserAgent returns the user agent string
func (s *AtlassianAuthService) GetUserAgent() string {
	return s.userAgent
}

// GetCloudID returns the Atlassian cloud ID
func (s *AtlassianAuthService) GetCloudID() string {
	return s.cloudId
}

// GetAtlToken returns the atl_token for CSRF protection
func (s *AtlassianAuthService) GetAtlToken() string {
	return s.atlToken
}
