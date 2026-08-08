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

	"github.com/ravshann/predlozhka/backend/internal/config"
	"github.com/ravshann/predlozhka/backend/internal/httpapi"
	"github.com/ravshann/predlozhka/backend/internal/store/postgres"
	"github.com/ravshann/predlozhka/backend/internal/youtube"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	youtubeClient := youtube.NewGoogleClient(cfg.YouTubeAPIKey)
	if cfg.YouTubeAPIKey != "" {
		go refreshYouTubeMetadata(ctx, database, youtubeClient, logger)
	}
	api := httpapi.New(cfg, database, youtubeClient, logger)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("api listening", "address", cfg.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGracePeriod)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func refreshYouTubeMetadata(
	ctx context.Context,
	database *postgres.Store,
	client youtube.Client,
	logger *slog.Logger,
) {
	refresh := func() {
		targets, err := database.ListYouTubeRefreshTargets(ctx, 100)
		if err != nil {
			logger.Error("youtube refresh list failed", "error", err)
			return
		}
		updated := 0
		for _, target := range targets {
			if ctx.Err() != nil {
				return
			}
			requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			metadata, err := client.Fetch(requestCtx, target.YouTubeID)
			cancel()
			if err != nil {
				logger.Warn("youtube metadata refresh failed", "youtube_id", target.YouTubeID, "error", err)
				continue
			}
			if err := database.UpdateYouTubeMetadata(
				ctx,
				target.SubmissionID,
				metadata.Title,
				metadata.ChannelTitle,
				metadata.ThumbnailURL,
				metadata.DurationSeconds,
				metadata.ViewCount,
				metadata.YouTubeLikeCount,
			); err != nil {
				logger.Error("youtube metadata update failed", "submission_id", target.SubmissionID, "error", err)
				continue
			}
			updated++
		}
		logger.Info("youtube metadata refresh completed", "updated", updated, "total", len(targets))
	}

	refresh()
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
