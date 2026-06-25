package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/open-xiv/memo-discord-bot/api"
	"github.com/open-xiv/memo-discord-bot/bot"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/logger"
	"github.com/open-xiv/memo-discord-bot/metrics"
	"github.com/open-xiv/memo-discord-bot/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func main() {
	logger.InitLogger()

	flow.InitDB()
	flow.InitRedis()

	// background goroutines — both cancelled on SIGINT/SIGTERM below.
	//   StartHealthRefresher: refreshes the /status dep snapshot every 15s
	//     so readiness probes are O(1).
	//   StartKeepalive: keeps one DB conn warm in the pool. mandatory for
	//     this service — see memo-docs/standards/observability.md
	//     "Dependency pool warmth".
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api.StartHealthRefresher(ctx)
	flow.StartKeepalive(ctx)

	// Serve health/metrics before connecting Discord: bot.Start() registers
	// slash commands synchronously and Discord rate-limits command writes, so
	// gating the :8080 bind on it would fail the liveness probe and crash-loop.
	go func() {
		r := gin.New()
		r.Use(gin.Recovery())
		r.Use(middleware.Logger())

		// HTTP request counter — labels (path, method, code).
		r.Use(func(c *gin.Context) {
			c.Next()
			metrics.HTTPRequestsTotal.WithLabelValues(
				c.FullPath(), c.Request.Method, strconv.Itoa(c.Writer.Status()),
			).Inc()
		})

		r.GET("/status", api.Status)
		r.GET("/status/live", api.StatusLive)
		r.GET("/status/ready", api.StatusReady)
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))

		api.RegisterWebhooks(r)

		if err := r.Run(":8080"); err != nil {
			log.Fatal().Msgf("failed to run server: %v", err)
		}
	}()

	flow.InitDiscord()
	bot.Start()
	defer bot.Stop()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
