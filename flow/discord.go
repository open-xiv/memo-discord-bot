package flow

import (
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

var Discord *discordgo.Session

func InitDiscord() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal().Msg("DISCORD_BOT_TOKEN not set")
	}

	var err error
	Discord, err = discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create discord session")
	}

	Discord.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
}
