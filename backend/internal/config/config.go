package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address                string
	BaseURL                string
	FrontendURL            string
	DatabaseURL            string
	SessionCookieName      string
	SessionTTL             time.Duration
	OwnerTwitchID          string
	OwnerTwitchLogin       string
	TwitchClientID         string
	TwitchClientSecret     string
	TwitchRedirectURL      string
	TwitchChatAccessToken  string
	TwitchChatRefreshToken string
	YouTubeAPIKey          string
	TurnstileSecretKey     string
	ShutdownGracePeriod    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Address:                env("API_ADDRESS", ":8080"),
		BaseURL:                env("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL:            env("FRONTEND_URL", "http://localhost:5173"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		SessionCookieName:      env("SESSION_COOKIE_NAME", "ravshann_session"),
		SessionTTL:             durationEnv("SESSION_TTL", 30*24*time.Hour),
		OwnerTwitchID:          os.Getenv("OWNER_TWITCH_ID"),
		OwnerTwitchLogin:       strings.ToLower(env("OWNER_TWITCH_LOGIN", "rafchibiskus")),
		TwitchClientID:         os.Getenv("TWITCH_CLIENT_ID"),
		TwitchClientSecret:     os.Getenv("TWITCH_CLIENT_SECRET"),
		TwitchRedirectURL:      os.Getenv("TWITCH_REDIRECT_URL"),
		TwitchChatAccessToken:  os.Getenv("TWITCH_CHAT_ACCESS_TOKEN"),
		TwitchChatRefreshToken: os.Getenv("TWITCH_CHAT_REFRESH_TOKEN"),
		YouTubeAPIKey:          os.Getenv("YOUTUBE_API_KEY"),
		TurnstileSecretKey:     os.Getenv("TURNSTILE_SECRET_KEY"),
		ShutdownGracePeriod:    durationEnv("SHUTDOWN_GRACE_PERIOD", 10*time.Second),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if !strings.HasPrefix(cfg.FrontendURL, "http://") && !strings.HasPrefix(cfg.FrontendURL, "https://") {
		return Config{}, errors.New("FRONTEND_URL must be an absolute http(s) URL")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
