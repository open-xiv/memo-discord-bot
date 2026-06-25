package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
)

func RegisterAdminHandlers() {
	CommandHandlers["set-hide"] = RequireRole(RoleDevID, "此命令仅限开发者")(handleForceHide)
}

// handleForceHide sets the privacy level on any member located by name+server,
// at any tier (公开/不上榜/隐藏), with no binding-ownership check — unlike /hide,
// which only toggles the caller's own bound members between 公开 and 不上榜.
// Dev-gated at registration.
func handleForceHide(c *Ctx) {
	i := c.I
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range i.ApplicationCommandData().Options {
		optionMap[opt.Name] = opt
	}

	name := strings.TrimSpace(optionMap["name"].StringValue())
	server := strings.TrimSpace(optionMap["server"].StringValue())
	level := model.Privacy(optionMap["level"].IntValue())
	if level < model.PrivacyPublic || level > model.PrivacyHidden {
		c.Error("无效的隐私等级")
		return
	}

	var member model.Member
	err := flow.DB.Where("name = ? AND server = ?", name, server).First(&member).Error
	if err != nil {
		c.Error("角色不存在")
		return
	}

	err = flow.DB.Model(&member).Update("privacy", int(level)).Error
	if err != nil {
		log.Error().Err(err).Str("name", member.Name).Str("server", member.Server).Msg("force privacy failed")
		c.Error("无法修改隐私状态 内部错误")
		return
	}

	c.Success(fmt.Sprintf("已将 %s@%s 设为 %s", member.Name, member.Server, privacyLabel(level)))
	log.Info().Str("operator", c.DiscordID()).Str("name", member.Name).Str("server", member.Server).Int("privacy", int(level)).Msg("force privacy success")
}
