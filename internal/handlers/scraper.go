package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bobmc/aktis-parser/internal/common"
	"github.com/bobmc/aktis-parser/internal/interfaces"
	"github.com/ternarybob/arbor"
)

type ScraperHandler struct {
	scraper   interfaces.Scraper
	logger    arbor.ILogger
	wsHandler *WebSocketHandler
}

func NewScraperHandler(s interfaces.Scraper, ws *WebSocketHandler) *ScraperHandler {
	return &ScraperHandler{
		scraper:   s,
		logger:    common.GetLogger(),
		wsHandler: ws,
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

	// Broadcast auth data to WebSocket clients
	if h.wsHandler != nil {
		h.wsHandler.BroadcastAuth(&authData)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "authenticated",
		"message": "Authentication captured successfully",
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

// ScrapeProjectsHandler triggers scraping of Jira projects only
func (h *ScraperHandler) ScrapeProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.scraper.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	go func() {
		if err := h.scraper.ScrapeProjects(); err != nil {
			h.logger.Error().Err(err).Msg("Project scrape error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Jira projects scraping started",
	})
}

// ScrapeSpacesHandler triggers scraping of Confluence spaces only
func (h *ScraperHandler) ScrapeSpacesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.scraper.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	go func() {
		if err := h.scraper.ScrapeConfluence(); err != nil {
			h.logger.Error().Err(err).Msg("Confluence scrape error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Confluence spaces scraping started",
	})
}
