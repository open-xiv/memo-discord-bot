package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/open-xiv/memo-discord-bot/buildinfo"
)

// InitLogger wires the global zerolog logger per memo-docs/standards/observability.md.
func InitLogger() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFieldName = "ts"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "level"
	zerolog.ErrorFieldName = "error"
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	zerolog.DurationFieldInteger = true
	zerolog.DurationFieldUnit = time.Millisecond

	level, err := zerolog.ParseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		level = zerolog.InfoLevel
	}

	log.Logger = zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Str("service", buildinfo.Service).
		Str("version", buildinfo.Version).
		Str("build", buildinfo.Build).
		Str("env", buildinfo.Env).
		Logger()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
