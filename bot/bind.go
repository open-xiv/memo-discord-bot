package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
)

const MaxBindings = 3

func RegisterBindHandlers() {
	CommandHandlers["bind"] = handleBind
	CommandHandlers["unbind"] = handleUnbind
	CommandHandlers["list"] = handleList
	CommandHandlers["hidden"] = handleHidden
}

func handleBind(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	name := strings.TrimSpace(optionMap["name"].StringValue())
	server := strings.TrimSpace(optionMap["server"].StringValue())
	discordID := i.Member.User.ID

	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).FirstOrCreate(&user, model.User{
		DiscordID: &discordID,
	}).Error

	if err != nil {
		log.Error().Err(err).Msg("user bind failed (create)")
		respondError(s, i, "无法绑定角色 内部错误")
		return
	}

	currentCount := flow.DB.Model(&user).Association("Members").Count()

	if currentCount >= MaxBindings {
		msg := fmt.Sprintf("已经绑定了 %d 个角色 请先使用 `/unbind` 解绑一个角色", MaxBindings)
		respondError(s, i, msg)
		return
	}

	var member model.Member
	err = flow.DB.Where("name = ? AND server = ?", name, server).First(&member).Error

	if err != nil {
		respondError(s, i, "角色不存在")
		return
	}

	var existingCount int64
	flow.DB.Table("user_members").
		Where("user_id = ? AND member_id = ?", user.ID, member.ID).
		Count(&existingCount)

	if existingCount > 0 {
		msg := fmt.Sprintf("已经绑定了 %s@%s", name, server)
		respondSuccess(s, i, msg)
		return
	}

	err = flow.DB.Model(&user).Association("Members").Append(&member)
	if err != nil {
		log.Error().Err(err).Str("name", name).Str("server", server).Msg("user bind failed (db)")
		respondError(s, i, "无法绑定角色 内部错误")
		return
	}

	log.Info().Str("discord_id", discordID).Str("name", name).Str("server", server).Msg("user bind success")
	msg := fmt.Sprintf("成功绑定了 %s@%s (%d / %d)", name, server, currentCount+1, MaxBindings)
	respondSuccess(s, i, msg)
}

func handleUnbind(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordID := i.Member.User.ID

	var user model.User
	err := flow.DB.Preload("Members").Where("discord_id = ?", discordID).First(&user).Error
	if err != nil || len(user.Members) == 0 {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	options := make([]discordgo.SelectMenuOption, len(user.Members))
	for idx, member := range user.Members {
		options[idx] = discordgo.SelectMenuOption{
			Label:       fmt.Sprintf("[%d] %s@%s", idx+1, member.Name, member.Server),
			Value:       fmt.Sprintf("%d", member.ID),
			Description: fmt.Sprintf("解绑 %s", member.Name),
		}
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "选择要解绑的角色",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "unbind_select",
							Placeholder: "请选择一名角色",
							Options:     options,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("user unbind failed")
	}
}

func handleList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordID := i.Member.User.ID

	var user model.User
	err := flow.DB.Preload("Members").Where("discord_id = ?", discordID).First(&user).Error
	if err != nil || len(user.Members) == 0 {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	fields := make([]*discordgo.MessageEmbedField, 0, len(user.Members))
	for idx, member := range user.Members {
		hiddenStatus := ""
		if member.Hidden {
			hiddenStatus = " 🔒"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%d. %s%s", idx+1, member.Name, hiddenStatus),
			Value:  fmt.Sprintf("%s", member.Server),
			Inline: true,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "已绑定的角色列表",
		Description: fmt.Sprintf("你已经绑定了 %d 名角色 (最多 %d 名)", len(user.Members), MaxBindings),
		Color:       0x00ff00,
		Fields:      fields,
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:  discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("user list failed")
	}
}

func handleHidden(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordID := i.Member.User.ID

	var user model.User
	err := flow.DB.Preload("Members").Where("discord_id = ?", discordID).First(&user).Error
	if err != nil || len(user.Members) == 0 {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	options := make([]discordgo.SelectMenuOption, len(user.Members))
	for idx, member := range user.Members {
		hiddenStatus := "显示"
		if member.Hidden {
			hiddenStatus = "隐藏"
		}
		options[idx] = discordgo.SelectMenuOption{
			Label:       fmt.Sprintf("[%d] %s@%s", idx+1, member.Name, member.Server),
			Value:       fmt.Sprintf("%d", member.ID),
			Description: fmt.Sprintf("当前状态: %s", hiddenStatus),
		}
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "选择要修改隐藏状态的角色",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "hidden_select",
							Placeholder: "请选择一名角色",
							Options:     options,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("user hidden select failed")
	}
}
