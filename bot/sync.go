package bot

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/lib/pq"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
)

var (
	roleCache      = make(map[string]*discordgo.Role)
	roleCacheMutex sync.RWMutex
	roleCacheTime  time.Time
)

func RegisterSyncHandlers() {
	flow.Discord.AddHandler(handleGuildMemberUpdate)
}

// Discord → Database
func handleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	// 1. filter by Guild ID
	if m.GuildID != GuildID {
		return
	}

	discordID := m.User.ID

	// 2. find user
	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).First(&user).Error
	if err != nil {
		return
	}

	// 3. filter roles
	var roleIDs []string
	for _, roleID := range m.Member.Roles {
		role := getRole(s, GuildID, roleID)
		if role != nil && !role.Managed {
			roleIDs = append(roleIDs, roleID)
		}
	}

	// 4. update database
	err = flow.DB.Model(&user).Update("role_ids", pq.StringArray(roleIDs)).Error
	if err != nil {
		log.Error().Err(err).Msgf("role sync failed [%s -> %s]", discordID, roleIDs)
		return
	}

	log.Info().Msgf("role sync success [%s -> %s]", discordID, roleIDs)
}

func getRole(s *discordgo.Session, guildID, roleID string) *discordgo.Role {
	if time.Since(roleCacheTime) > 5*time.Minute {
		refreshRoleCache(s, guildID)
	}

	roleCacheMutex.RLock()
	defer roleCacheMutex.RUnlock()
	return roleCache[roleID]
}

func refreshRoleCache(s *discordgo.Session, guildID string) {
	guild, err := s.Guild(guildID)
	if err != nil {
		log.Error().Err(err).Msg("role cache refresh failed")
		return
	}

	roleCacheMutex.Lock()
	defer roleCacheMutex.Unlock()

	roleCache = make(map[string]*discordgo.Role)
	for _, role := range guild.Roles {
		roleCache[role.ID] = role
	}
	roleCacheTime = time.Now()
}
