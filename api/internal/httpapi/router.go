package httpapi

import (
	"net/http"
)

// NewRouter creates and returns a new HTTP router with all the necessary routes registered.
func NewRouter() http.Handler {
	// Create a new HTTP request multiplexer.
	mux := http.NewServeMux()

	// Register the health check endpoint.
	mux.HandleFunc("GET /healthz", HealthHandler)

	// Additional routes can be registered here.
	return requestID(logRequests(recoverer(mux)))
}
