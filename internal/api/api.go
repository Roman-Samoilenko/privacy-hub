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

const (
	APITimeout      = 60 * time.Second
	ShutdownTimeout = 5 * time.Second
)

func CreateRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(APITimeout))

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

	logger.Infof("Starting HTTP server on %s", server.Addr)

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Server error: %v", err)
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Infof("Shutting down API server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
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
		logger.Infof("HTTP %d %s %s completed in %v",
			ww.Status(),
			r.Method,
			r.URL.Path,
			duration,
		)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		logger.Errorf("Failed to write health response: %v", err)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Current configuration"))
	if err != nil {
		logger.Errorf("Failed to write config response: %v", err)
	}
}

func restartHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Service is restarting"))
	if err != nil {
		logger.Errorf("Failed to write restart response: %v", err)
	}
	logger.Warnf("Restart endpoint hit - service restarting")
}

var ReadyChan = make(chan bool)

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("The container is ready"))
	if err != nil {
		logger.Errorf("Failed to write readiness response: %v", err)
	}
	logger.Infof("Readiness check passed")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Failed to read readiness request body: %v", err)
		return
	}

	logger.Debugf("Readiness request body: %s", string(body))

	var data map[string]bool
	err = json.Unmarshal(body, &data)
	if err != nil {
		logger.Errorf("Failed to parse readiness request body: %v", err)
		return
	}

	ready, exists := data["ready"]
	if !exists {
		logger.Errorf("Readiness request body does not contain 'ready' field")
		return
	}

	ReadyChan <- ready
}
