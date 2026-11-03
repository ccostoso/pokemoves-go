package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON writes the given body as a JSON response with the specified HTTP status code.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}

// writeError writes an error response as JSON with the specified HTTP status code, error code, and message.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}
