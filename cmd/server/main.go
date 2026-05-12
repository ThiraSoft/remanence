package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"remanence/internal/about"
	"remanence/internal/business"
	"remanence/internal/config"
	"remanence/internal/frontend"
	"remanence/internal/health"
	"remanence/internal/logger"
	"remanence/internal/messages"
	"remanence/internal/middleware"
)

var (
	version   string
	buildDate string
)

func main() {
	mux := http.NewServeMux()

	// Per-IP rate limiter: 20 requests per minute
	middleware.InitRateLimiter(20, 1*time.Minute)

	// Background cleanup of expired messages (every 5 minutes)
	messages.StartCleanup(5 * time.Minute)

	// Health check endpoints (no middleware needed)
	mux.HandleFunc("/health/live", health.HealthLiveHandler())
	mux.HandleFunc("/health/ready", health.HealthReadyHandler())
	mux.HandleFunc("/health", health.HealthAggregateHandler())

	// About
	mux.HandleFunc("/api/v1/about", about.HandleAboutEndpoint)

	// Business endpoints with middleware chain
	mux.HandleFunc("/api/v1/messages/{id}",
		middleware.Chain(
			business.GetMessageByID,
			middleware.SecurityHeaders,
			middleware.WithRequestID,
			middleware.RateLimitByIP,
		),
	)

	mux.HandleFunc("/api/v1/messages",
		middleware.Chain(
			business.CreateMessage,
			middleware.SecurityHeaders,
			middleware.WithRequestID,
			middleware.RateLimitByIP,
		),
	)

	// Serve frontend
	mux.Handle("/", frontend.Handler())
	mux.Handle("/about", frontend.AboutHandler())

	// Start server
	logger.Log.Info(
		"==== Server listening ====",
		slog.String("url", "127.0.0.1"),
		slog.String("port", config.Port),
	)
	srv := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           mux,
		ReadTimeout:       config.Timeout,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      config.Timeout,
		IdleTimeout:       config.Timeout,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Log.Info("Received signal, shutting down gracefully...", slog.String("signal", sig.String()))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Log.Error("Graceful shutdown failed", slog.String("error", err.Error()))
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	logger.Log.Info("Server stopped")
}
