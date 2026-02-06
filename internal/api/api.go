package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func CreateRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(loggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", healthHandler)
	r.Get("/config", configHandler)
	r.Post("/restart", restartHandler)

	return r
}

func Start(ctx context.Context, cfg config.APIConfig) error {
	server := &http.Server{
		Addr:         cfg.Listen,
		Handler:      CreateRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Infof("Starting API server on %s", server.Addr)

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Infof("Shutting down API server...")
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
		logger.Infof("HTTP %d %s %s (%v)",
			ww.Status(), r.Method, r.URL.Path, time.Since(start))
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Configuration endpoint"))
}

func restartHandler(w http.ResponseWriter, r *http.Request) {
	logger.Warnf("Restart requested via API")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Restart initiated",
	})
}
