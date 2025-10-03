package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/ternarybob/arbor"
)

// CollectorHandler handles API endpoints for the collector service
type CollectorHandler struct {
	jiraScraper       JiraDataProvider
	confluenceScraper ConfluenceDataProvider
	logger            arbor.ILogger
}

// JiraDataProvider interface for accessing Jira data
type JiraDataProvider interface {
	GetJiraData() (map[string]interface{}, error)
}

// ConfluenceDataProvider interface for accessing Confluence data
type ConfluenceDataProvider interface {
	GetConfluenceData() (map[string]interface{}, error)
}

// PaginationResponse contains pagination metadata
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// CollectorResponse is the standard response format
type CollectorResponse struct {
	Data       interface{}        `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewCollectorHandler creates a new collector handler
func NewCollectorHandler(jiraScraper JiraDataProvider, confluenceScraper ConfluenceDataProvider, logger arbor.ILogger) *CollectorHandler {
	return &CollectorHandler{
		jiraScraper:       jiraScraper,
		confluenceScraper: confluenceScraper,
		logger:            logger,
	}
}

// GetProjectsHandler returns paginated list of projects with counts
func (h *CollectorHandler) GetProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, pageSize := h.getPaginationParams(r)

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira data")
		http.Error(w, "Failed to get projects", http.StatusInternalServerError)
		return
	}

	projects, ok := data["projects"].([]map[string]interface{})
	if !ok {
		projects = []map[string]interface{}{}
	}

	// Add issue count to each project
	issues, _ := data["issues"].([]map[string]interface{})
	projectIssueCounts := make(map[string]int)
	for _, issue := range issues {
		if fields, ok := issue["fields"].(map[string]interface{}); ok {
			if project, ok := fields["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok {
					projectIssueCounts[key]++
				}
			}
		}
	}

	// Enrich projects with issue counts
	for i := range projects {
		if key, ok := projects[i]["key"].(string); ok {
			projects[i]["issueCount"] = projectIssueCounts[key]
		}
	}

	paginatedData, pagination := h.paginate(projects, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSpacesHandler returns paginated list of Confluence spaces with page counts
func (h *CollectorHandler) GetSpacesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, pageSize := h.getPaginationParams(r)

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence data")
		http.Error(w, "Failed to get spaces", http.StatusInternalServerError)
		return
	}

	spaces, ok := data["spaces"].([]map[string]interface{})
	if !ok {
		spaces = []map[string]interface{}{}
	}

	paginatedData, pagination := h.paginate(spaces, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetIssuesHandler returns paginated list of issues for a project
func (h *CollectorHandler) GetIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectKey := r.URL.Query().Get("projectKey")
	if projectKey == "" {
		http.Error(w, "projectKey parameter required", http.StatusBadRequest)
		return
	}

	page, pageSize := h.getPaginationParams(r)

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira data")
		http.Error(w, "Failed to get issues", http.StatusInternalServerError)
		return
	}

	allIssues, ok := data["issues"].([]map[string]interface{})
	if !ok {
		allIssues = []map[string]interface{}{}
	}

	// Filter issues by project key
	var filteredIssues []map[string]interface{}
	for _, issue := range allIssues {
		if fields, ok := issue["fields"].(map[string]interface{}); ok {
			if project, ok := fields["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok && key == projectKey {
					filteredIssues = append(filteredIssues, issue)
				}
			}
		}
	}

	paginatedData, pagination := h.paginate(filteredIssues, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPagesHandler returns paginated list of pages for a space
func (h *CollectorHandler) GetPagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spaceKey := r.URL.Query().Get("spaceKey")
	if spaceKey == "" {
		http.Error(w, "spaceKey parameter required", http.StatusBadRequest)
		return
	}

	page, pageSize := h.getPaginationParams(r)

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence data")
		http.Error(w, "Failed to get pages", http.StatusInternalServerError)
		return
	}

	allPages, ok := data["pages"].([]map[string]interface{})
	if !ok {
		allPages = []map[string]interface{}{}
	}

	// Filter pages by space key
	var filteredPages []map[string]interface{}
	for _, page := range allPages {
		if space, ok := page["space"].(map[string]interface{}); ok {
			if key, ok := space["key"].(string); ok && key == spaceKey {
				filteredPages = append(filteredPages, page)
			}
		}
	}

	paginatedData, pagination := h.paginate(filteredPages, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getPaginationParams extracts page and pageSize from query params
func (h *CollectorHandler) getPaginationParams(r *http.Request) (int, int) {
	page := 0
	pageSize := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	return page, pageSize
}

// paginate applies pagination to a slice of data
func (h *CollectorHandler) paginate(data []map[string]interface{}, page, pageSize int) ([]map[string]interface{}, PaginationResponse) {
	totalItems := len(data)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	start := page * pageSize
	end := start + pageSize

	if start >= totalItems {
		return []map[string]interface{}{}, PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		}
	}

	if end > totalItems {
		end = totalItems
	}

	paginatedData := data[start:end]

	pagination := PaginationResponse{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}

	return paginatedData, pagination
}
