package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bobmc/aktis-parser/internal/common"
	"github.com/bobmc/aktis-parser/internal/interfaces"
	"github.com/ternarybob/arbor"
)

type DataHandler struct {
	scraper interfaces.Scraper
	logger  arbor.ILogger
}

func NewDataHandler(s interfaces.Scraper) *DataHandler {
	return &DataHandler{
		scraper: s,
		logger:  common.GetLogger(),
	}
}

// GetJiraDataHandler returns all Jira data (projects and issues)
func (h *DataHandler) GetJiraDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.scraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		http.Error(w, "Failed to fetch Jira data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetConfluenceDataHandler returns all Confluence data (pages)
func (h *DataHandler) GetConfluenceDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.scraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		http.Error(w, "Failed to fetch Confluence data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
