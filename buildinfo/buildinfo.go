package buildinfo

import (
	"os"
	"time"
)

const Service = "memo-discord-bot"

var (
	Version   = envOr("MEMO_VERSION", "dev")
	Build     = envOr("MEMO_BUILD", "unknown")
	Env       = envOr("MEMO_ENV", "dev")
	StartedAt = time.Now().UTC()
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
