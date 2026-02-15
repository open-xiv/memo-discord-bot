package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/open-xiv/memo-discord-bot/api"
	"github.com/open-xiv/memo-discord-bot/bot"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/logger"
	"github.com/rs/zerolog/log"
)

func main() {
	// logger
	logger.InitLogger()

	// database
	flow.InitDB()
	flow.InitRedis()

	// discord bot
	flow.InitDiscord()
	bot.Start()
	defer bot.Stop()

	// restful
	go func() {
		r := gin.New()
		r.Use(gin.Recovery())

		r.GET("/", api.Status)
		r.GET("/status", api.Status)

		if err := r.Run(":8080"); err != nil {
			log.Fatal().Msgf("failed to run server: %v", err)
		}
	}()

	// wait for termination signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
