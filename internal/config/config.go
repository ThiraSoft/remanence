package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// App settings
	AppVersion = getEnv("APP_VERSION", "undefined")
	BuildDate  = getEnv("BUILD_DATE", time.Now().Format("2006-01-02T15:04:05-07:00"))
	Port       = getEnv("PORT", "8080")
	Timeout    = 30 * time.Second
	LogLevel   = getEnv("LOG_LEVEL", "ERROR") // Log level: DEBUG, INFO, WARN, ERROR

	// Set to "true" when behind a reverse proxy (nginx, cloudflare, etc.)
	// to trust X-Forwarded-For for rate limiting. Never enable on direct exposure.
	TrustProxy = strings.EqualFold(getEnv("TRUST_PROXY", "false"), "true")

	// Message settings
	MessageIDLength  = getEnvInt("MESSAGE_ID_LENGTH", 16)
	MaxMessageLength = getEnvInt("MAX_MESSAGE_LENGTH", 1024)
	MaxLifeLimit     = 1440 // in minutes (24 hours)
)

func ParseLogLevel() slog.Level {
	switch strings.ToUpper(LogLevel) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func getEnvInt(k string, d int) int {
	v := getEnv(k, "")
	if v == "" {
		return d
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	return i
}
