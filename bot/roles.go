package bot

import (
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/rs/zerolog/log"
)

func AddRoleToUser(userID, roleID string) error {
	err := flow.Discord.GuildMemberRoleAdd(GuildID, userID, roleID)
	if err != nil {
		log.Error().Err(err).Str("discord_id", userID).Str("role_id", roleID).Msg("role bind failed")
		return err
	}
	log.Info().Str("discord_id", userID).Str("role_id", roleID).Msg("role bind success")
	return nil
}
