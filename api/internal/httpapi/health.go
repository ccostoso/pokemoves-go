package httpapi

import (
	"net/http"
)

// HealthHandler handles the health check endpoint and returns a JSON response indicating the service status.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
