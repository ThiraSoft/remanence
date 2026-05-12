package logger

import (
	"log/slog"
	"os"

	"remanence/internal/config"
)

var Log *slog.Logger

func init() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.ParseLogLevel(),
	})
	Log = slog.New(handler)
}
