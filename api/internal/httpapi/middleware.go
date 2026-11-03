package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type loggerKey struct{}

func loggerFrom(ctx context.Context) *slog.Logger {
	// Retrieve the logger from the context if it exists.
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return nil
}

// newRequestID generates a new random request ID as a hexadecimal string.
func newRequestID() string {
	// Create a byte slice to hold the random request ID.
	b := make([]byte, 8)

	// Fill the byte slice with random data.
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

// requestID is a middleware that generates a new request ID for each incoming HTTP request,
// sets it in the response header, and adds it to the request context along with a logger containing the request ID.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a new request ID.
		id := newRequestID()

		// Set the request ID in the response header.
		w.Header().Set("X-Request-ID", id)

		// Create a logger with the request ID as a field.
		logger := slog.Default().With("request_id", id)

		// Add the request ID to the request context.
		ctx := context.WithValue(r.Context(), loggerKey{}, logger)

		// Call the next handler with the updated request.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder is a wrapper around http.ResponseWriter that records the HTTP status code of the response.
// It is used to capture the status code for logging or metrics purposes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code and calls the underlying ResponseWriter's WriteHeader method.
func (s *statusRecorder) WriteHeader(code int) {
	// Record the status code.
	s.status = code

	// Call the underlying ResponseWriter's WriteHeader method.
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the underlying http.ResponseWriter.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record the start time of the request.
		start := time.Now()

		// Wrap the ResponseWriter with a statusRecorder to capture the status code.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		// Call the next handler with the wrapped ResponseWriter.
		next.ServeHTTP(rec, r)

		// Log the request details using the logger from the context.
		loggerFrom(r.Context()).Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Recover from panics and return a 500 Internal Server Error if necessary.
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					// If the panic is due to an aborted handler, just re-panic to let the server handle it.
					panic(p)
				}
				loggerFrom(r.Context()).Error("panic recovered",
					"panic", p,
					"stack", string(debug.Stack()),
				)
				writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong.")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
