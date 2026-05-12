package main

import (
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

	flow.InitDiscord()
	bot.Start()
	defer bot.Stop()

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

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
