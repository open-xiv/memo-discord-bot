package flow

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	Redis *redis.Client
	Cache = context.Background()
)

func InitRedis() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
		log.Warn().Msgf("REDIS_URL not set, using %s", url)
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse REDIS_URL")
	}

	Redis = redis.NewClient(opts)

	opts.PoolSize = 60
	opts.MinIdleConns = 10
	opts.PoolTimeout = 3 * time.Second

	_, err = Redis.Ping(Cache).Result()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect REDIS_URL")
	}
}
