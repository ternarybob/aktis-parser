package handlers

import (
	"encoding/json"
	"net/http"

	"aktis-parser/internal/common"
	"github.com/ternarybob/arbor"
)

type APIHandler struct {
	logger arbor.ILogger
}

func NewAPIHandler() *APIHandler {
	return &APIHandler{
		logger: common.GetLogger(),
	}
}

// VersionHandler returns version information
func (h *APIHandler) VersionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version":    common.GetVersion(),
		"build":      common.GetBuild(),
		"git_commit": common.GetGitCommit(),
	})
}

// HealthHandler returns health check status
func (h *APIHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// NotFoundHandler handles 404 errors with JSON response
func (h *APIHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "Not Found",
		"path":    r.URL.Path,
		"message": "The requested endpoint does not exist",
	})
}
