package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/open-xiv/memo-discord-bot/service/fflogs"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func RegisterLogsHandlers() {
	CommandHandlers["logs"] = handleLogs
}

func validateKey(clientID, clientSecret string) error {
	client := fflogs.NewLogsClient(clientID, clientSecret)

	ctx := context.Background()
	_, err := client.GetRateLimitData(ctx)

	return err
}

func handleLogs(c *Ctx) {
	s, i := c.S, c.I
	discordID := c.DiscordID()

	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).First(&user).Error
	if err != nil {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	var existingKey model.LogsKey
	err = flow.DB.Where("user_id = ?", user.ID).First(&existingKey).Error

	if err == nil {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("目前有同步服务 [%s] \n\n 需要更新吗", maskClientID(existingKey.Client)),
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "更新",
								Style:    discordgo.PrimaryButton,
								CustomID: "logs_update",
							},
							discordgo.Button{
								Label:    "取消",
								Style:    discordgo.SecondaryButton,
								CustomID: "logs_cancel",
							},
						},
					},
				},
			},
		})
		if err != nil {
			return
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		showLogsModal(c, false)
	} else {
		log.Error().Err(err).Msg("logs key bind failed (create)")
		respondError(s, i, "无法绑定同步服务 内部错误")
	}
}

func showLogsModal(c *Ctx, isUpdate bool) {
	s, i := c.S, c.I
	title := "设置 FFLogs 同步服务"
	if isUpdate {
		title = "更新 FFLogs 同步服务"
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "logs_modal",
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "client_id",
							Label:       "FFLogs Client ID",
							Style:       discordgo.TextInputShort,
							Placeholder: "FFLogs OAuth Client ID (v2)",
							Required:    true,
							MinLength:   36,
							MaxLength:   36,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "client_secret",
							Label:       "FFLogs Client Secret",
							Style:       discordgo.TextInputShort,
							Placeholder: "FFLogs OAuth Client Secret (v2)",
							Required:    true,
							MinLength:   40,
							MaxLength:   40,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("logs key bind failed (modal)")
		respondError(s, i, "无法绑定同步服务 内部错误")
	}
}

func handleLogsModal(c *Ctx) {
	s, i := c.S, c.I
	data := i.ModalSubmitData()

	var clientID, clientSecret string
	for _, component := range data.Components {
		if actionRow, ok := component.(*discordgo.ActionsRow); ok {
			for _, comp := range actionRow.Components {
				if textInput, ok := comp.(*discordgo.TextInput); ok {
					switch textInput.CustomID {
					case "client_id":
						clientID = textInput.Value
					case "client_secret":
						clientSecret = textInput.Value
					}
				}
			}
		}
	}

	if clientID == "" || clientSecret == "" {
		respondError(s, i, "请提供有效的 Client ID 和 Client Secret")
		return
	}

	discordID := c.DiscordID()

	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).First(&user).Error
	if err != nil {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	var existingKey model.LogsKey
	err = flow.DB.Where("client = ? AND user_id != ?", clientID, user.ID).First(&existingKey).Error
	if err == nil {
		respondError(s, i, "此 Client ID 已经被其他用户绑定")
		return
	}

	err = validateKey(clientID, clientSecret)
	if err != nil {
		respondError(s, i, "无效的 FFLogs API 凭据")
		return
	}

	logsKey := model.LogsKey{
		UserID: user.ID,
		Client: clientID,
		Secret: clientSecret,
	}

	err = flow.DB.Where("user_id = ?", user.ID).Assign(logsKey).FirstOrCreate(&logsKey).Error
	if err != nil {
		log.Error().Err(err).Msg("logs key bind failed (db)")
		respondError(s, i, "无法绑定同步服务 内部错误")
		return
	}

	log.Info().Str("discord_id", discordID).Str("client_id_masked", maskClientID(clientID)).Msg("logs key bind success")

	err = AddRoleToUser(discordID, RoleLogsBindID)
	if err != nil {
		log.Error().Err(err).Msg("logs key bind failed (role)")
		respondError(s, i, "无法绑定同步服务 内部错误")
		return
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✅ 成功绑定 FFLogs 同步服务",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

func handleLogsButton(c *Ctx) {
	s, i := c.S, c.I
	data := i.MessageComponentData()

	switch data.CustomID {
	case "logs_update":
		showLogsModal(c, true)
	case "logs_cancel":
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "操作已取消",
				Components: []discordgo.MessageComponent{},
			},
		})
		if err != nil {
			return
		}
	}
}

func maskClientID(clientID string) string {
	if len(clientID) <= 8 {
		return clientID[:4]
	}
	return clientID[:8]
}
