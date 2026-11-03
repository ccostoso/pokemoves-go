package httpapi

import (
	"net/http"

	"github.com/ccostoso/pokemoves-go/api/internal/openapi"
)

// HealthHandler handles the health check endpoint and returns a JSON response indicating the service status.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, openapi.Health{Status: "ok"})
}
