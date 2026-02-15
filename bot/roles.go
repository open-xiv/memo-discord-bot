package bot

import (
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/rs/zerolog/log"
)

func AddRoleToUser(userID, roleID string) error {
	err := flow.Discord.GuildMemberRoleAdd(GuildID, userID, roleID)
	if err != nil {
		log.Error().Err(err).Msgf("role bind failed [%s -> %s]", userID, roleID)
		return err
	}
	log.Info().Msgf("role bind success [%s -> %s]", userID, roleID)
	return nil
}
