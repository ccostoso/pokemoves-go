package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ccostoso/pokemoves-go/api/internal/config"
	"github.com/ccostoso/pokemoves-go/api/internal/httpapi"
)

func main() {
	// Set up structured logging with JSON output.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load the application configuration.
	cfg := config.Load()

	// Initialize the HTTP server with the configured address and router.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Create a context that is canceled on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start the server in a separate goroutine.
	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			stop()
		}
	}()

	// Wait for the context to be canceled (on SIGINT or SIGTERM).
	<-ctx.Done()
	slog.Info("shutting down")

	// Create a context with a timeout for the server shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	} else {
		slog.Info("server exited properly")
	}
}
