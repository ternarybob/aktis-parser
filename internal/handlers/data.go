package handlers

import (
	"encoding/json"
	"net/http"

	"aktis-parser/internal/common"
	"aktis-parser/internal/interfaces"
	"github.com/ternarybob/arbor"
)

type DataHandler struct {
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
	logger            arbor.ILogger
}

func NewDataHandler(jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper) *DataHandler {
	return &DataHandler{
		jiraScraper:       jira,
		confluenceScraper: confluence,
		logger:            common.GetLogger(),
	}
}

// GetJiraDataHandler returns all Jira data (projects and issues)
func (h *DataHandler) GetJiraDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		http.Error(w, "Failed to fetch Jira data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetJiraIssuesHandler returns issues optionally filtered by project keys
func (h *DataHandler) GetJiraIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get optional project keys filter from query params
	projectKeys := r.URL.Query()["projectKey"]

	h.logger.Info().Strs("projectKeys", projectKeys).Msg("GetJiraIssuesHandler called")

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		http.Error(w, "Failed to fetch Jira data", http.StatusInternalServerError)
		return
	}

	issues := data["issues"]
	if issues == nil {
		issues = []interface{}{}
	}

	totalIssuesInDB := 0
	// Handle both []interface{} and []map[string]interface{} types
	if issueList, ok := issues.([]interface{}); ok {
		totalIssuesInDB = len(issueList)
	} else if issueList, ok := issues.([]map[string]interface{}); ok {
		totalIssuesInDB = len(issueList)
	}

	// Filter by project keys if specified
	if len(projectKeys) > 0 {
		filteredIssues := []interface{}{}

		// Handle []map[string]interface{} type from GetJiraData()
		if issueList, ok := issues.([]map[string]interface{}); ok {
			for _, issue := range issueList {
				if fields, ok := issue["fields"].(map[string]interface{}); ok {
					if project, ok := fields["project"].(map[string]interface{}); ok {
						if key, ok := project["key"].(string); ok {
							for _, pk := range projectKeys {
								if key == pk {
									filteredIssues = append(filteredIssues, issue)
									break
								}
							}
						}
					}
				}
			}
			issues = filteredIssues
		} else if issueList, ok := issues.([]interface{}); ok {
			for _, issue := range issueList {
				if issueMap, ok := issue.(map[string]interface{}); ok {
					if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
						if project, ok := fields["project"].(map[string]interface{}); ok {
							if key, ok := project["key"].(string); ok {
								for _, pk := range projectKeys {
									if key == pk {
										filteredIssues = append(filteredIssues, issue)
										break
									}
								}
							}
						}
					}
				}
			}
			issues = filteredIssues
		}
		firstProjectKey := "none"
		if len(filteredIssues) > 0 {
			if issueMap, ok := filteredIssues[0].(map[string]interface{}); ok {
				if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
					if project, ok := fields["project"].(map[string]interface{}); ok {
						if key, ok := project["key"].(string); ok {
							firstProjectKey = key
						}
					}
				}
			}
		}

		h.logger.Info().
			Int("totalInDB", totalIssuesInDB).
			Int("filtered", len(filteredIssues)).
			Strs("projectKeys", projectKeys).
			Str("firstIssueProject", firstProjectKey).
			Msg("Filtered issues by project")
	}

	// Log what we're returning
	returnCount := 0
	if issueList, ok := issues.([]interface{}); ok {
		returnCount = len(issueList)
	}
	h.logger.Info().
		Int("returningIssueCount", returnCount).
		Strs("requestedProjects", projectKeys).
		Msg("Returning issues to client")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issues": issues,
	})
}

// GetConfluenceDataHandler returns all Confluence data (spaces and pages)
func (h *DataHandler) GetConfluenceDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		http.Error(w, "Failed to fetch Confluence data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetConfluencePagesHandler returns pages optionally filtered by space keys
func (h *DataHandler) GetConfluencePagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spaceKeys := r.URL.Query()["spaceKey"]

	h.logger.Info().Strs("spaceKeys", spaceKeys).Msg("GetConfluencePagesHandler called")

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		http.Error(w, "Failed to fetch Confluence data", http.StatusInternalServerError)
		return
	}

	pages := data["pages"]
	if pages == nil {
		pages = []interface{}{}
	}

	if len(spaceKeys) > 0 {
		filteredPages := []interface{}{}

		if pageList, ok := pages.([]map[string]interface{}); ok {
			for _, page := range pageList {
				if space, ok := page["space"].(map[string]interface{}); ok {
					if key, ok := space["key"].(string); ok {
						for _, sk := range spaceKeys {
							if key == sk {
								filteredPages = append(filteredPages, page)
								break
							}
						}
					}
				}
			}
			pages = filteredPages
		} else if pageList, ok := pages.([]interface{}); ok {
			for _, page := range pageList {
				if pageMap, ok := page.(map[string]interface{}); ok {
					if space, ok := pageMap["space"].(map[string]interface{}); ok {
						if key, ok := space["key"].(string); ok {
							for _, sk := range spaceKeys {
								if key == sk {
									filteredPages = append(filteredPages, page)
									break
								}
							}
						}
					}
				}
			}
			pages = filteredPages
		}

		h.logger.Info().
			Int("filtered", len(filteredPages)).
			Strs("spaceKeys", spaceKeys).
			Msg("Filtered pages by space")
	}

	returnCount := 0
	if pageList, ok := pages.([]interface{}); ok {
		returnCount = len(pageList)
	}
	h.logger.Info().
		Int("returningPageCount", returnCount).
		Strs("requestedSpaces", spaceKeys).
		Msg("Returning pages to client")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pages": pages,
	})
}
