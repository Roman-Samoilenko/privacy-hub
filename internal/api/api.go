// api/api.go
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// CreateRouter создает и настраивает роутер
func CreateRouter() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Routes
	r.Get("/health", healthHandler)
	r.Get("/config", configHandler)
	r.Post("/ready", readyHandler)
	r.Post("/restart", restartHandler)

	return r
}

func Start(ctx context.Context, apiConf config.APIConfig) error {
	server := &http.Server{
		Addr:    apiConf.Listen,
		Handler: CreateRouter(),
	}

	logger.Info("Starting HTTP server on %s", server.Addr)

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Info("Shutting down API server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		duration := time.Since(start)
		logger.Info("HTTP %d %s %s completed in %v",
			ww.Status(),
			r.Method,
			r.URL.Path,
			duration,
		)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Current configuration"))
}

func restartHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Service is restarting"))
	logger.Warn("Restart endpoint hit - service restarting")
}

var ReadyChan = make(chan bool)

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("The container is ready"))
	logger.Info("Readiness check passed")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read readiness request body: %v", err)
		return
	}

	logger.Debug("Readiness request body: %s", string(body))

	var data map[string]bool
	err = json.Unmarshal(body, &data)
	if err != nil {
		logger.Error("Failed to parse readiness request body: %v", err)
		return
	}

	ready, exists := data["ready"]
	if !exists {
		logger.Error("Readiness request body does not contain 'ready' field")
		return
	}

	ReadyChan <- ready
}
