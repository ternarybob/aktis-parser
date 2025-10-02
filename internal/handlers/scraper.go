package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bobmc/aktis-parser/internal/common"
	"github.com/bobmc/aktis-parser/internal/interfaces"
	"github.com/ternarybob/arbor"
)

type ScraperHandler struct {
	scraper interfaces.Scraper
	logger  arbor.ILogger
}

func NewScraperHandler(s interfaces.Scraper) *ScraperHandler {
	return &ScraperHandler{
		scraper: s,
		logger:  common.GetLogger(),
	}
}

// AuthUpdateHandler handles authentication updates from Chrome extension
func (h *ScraperHandler) AuthUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var authData interfaces.AuthData
	if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode auth data")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.scraper.UpdateAuth(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update auth")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Start scraping in background
	go func() {
		if err := h.scraper.ScrapeAll(); err != nil {
			h.logger.Error().Err(err).Msg("Scrape error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "authenticated",
		"message": "Scraping started",
	})
}

// ScrapeHandler manually triggers scraping
func (h *ScraperHandler) ScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go h.scraper.ScrapeAll()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Scraping triggered",
	})
}
