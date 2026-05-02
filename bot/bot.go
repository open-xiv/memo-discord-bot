package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/metrics"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
)

var CommandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){}

func Start() {
	s := flow.Discord

	// handlers
	RegisterBindHandlers()
	RegisterLogsHandlers()
	RegisterSyncHandlers()

	// pre-create metric label series at 0 so PromQL increase() over a window
	// has both endpoints from the start — without this the series is born at
	// value=1 on first use and `increase()` returns 0 until the second call.
	warmInteractionLabels()
	warmSessionEventLabels()

	// component handlers
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			// slash
			name := i.ApplicationCommandData().Name
			metrics.InteractionsTotal.WithLabelValues("app_command", name).Inc()
			if h, ok := CommandHandlers[name]; ok {
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			// components
			metrics.InteractionsTotal.WithLabelValues("component", "").Inc()
			handleComponentInteraction(s, i)
		case discordgo.InteractionModalSubmit:
			// modal
			metrics.InteractionsTotal.WithLabelValues("modal", "").Inc()
			handleModalSubmit(s, i)
		case discordgo.InteractionApplicationCommandAutocomplete:
			// autocomplete
			metrics.InteractionsTotal.WithLabelValues("autocomplete", i.ApplicationCommandData().Name).Inc()
			handleAutocomplete(s, i)
		}
	})

	// register ready handler
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		metrics.SessionEvents.WithLabelValues("ready").Inc()
		log.Info().Msgf("discord bot session start (%s)", r.User.String())
	})

	// gateway resilience signals — useful when the bot stops responding,
	// usually correlates with a Disconnect spike.
	s.AddHandler(func(s *discordgo.Session, _ *discordgo.Disconnect) {
		metrics.SessionEvents.WithLabelValues("disconnect").Inc()
	})
	s.AddHandler(func(s *discordgo.Session, _ *discordgo.Resumed) {
		metrics.SessionEvents.WithLabelValues("resume").Inc()
	})
	s.AddHandler(func(s *discordgo.Session, e *discordgo.RateLimit) {
		metrics.SessionEvents.WithLabelValues("rate_limited").Inc()
		log.Warn().Str("url", e.URL).Msg("discord rate limit hit")
	})

	// connect to Discord
	err := s.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("discord bot connect failed")
	}

	// slash commands
	removeGlobalCommands(s)
	registerCommands(s)
}

func warmInteractionLabels() {
	// type=app_command,autocomplete — keyed by command name
	for _, cmd := range Commands {
		metrics.InteractionsTotal.WithLabelValues("app_command", cmd.Name).Add(0)
		metrics.InteractionsTotal.WithLabelValues("autocomplete", cmd.Name).Add(0)
	}
	// type=component,modal — name is always empty in our dispatcher
	metrics.InteractionsTotal.WithLabelValues("component", "").Add(0)
	metrics.InteractionsTotal.WithLabelValues("modal", "").Add(0)
}

func warmSessionEventLabels() {
	for _, ev := range []string{"ready", "disconnect", "resume", "rate_limited"} {
		metrics.SessionEvents.WithLabelValues(ev).Add(0)
	}
}

func removeGlobalCommands(s *discordgo.Session) {
	commands, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		log.Error().Err(err).Msg("global command fetch failed")
		return
	}

	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
		if err != nil {
			log.Error().Err(err).Msgf("global command delete failed (%s)", cmd.Name)
		} else {
			log.Info().Msgf("global command deleted (%s)", cmd.Name)
		}
	}
}

func registerCommands(s *discordgo.Session) {
	for _, cmd := range Commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, GuildID, cmd)
		if err != nil {
			log.Error().Err(err).Msgf("discord bot command registration failed (%s)", cmd.Name)
		} else {
			log.Info().Err(err).Msgf("discord bot command register (%s)", cmd.Name)
		}
	}
}

func handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	switch data.CustomID {
	case "unbind_select":
		handleUnbindSelect(s, i)
	case "hidden_select":
		handleHiddenSelect(s, i)
	case "logs_update", "logs_cancel":
		handleLogsButton(s, i)
	}
}

func handleUnbindSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	if len(data.Values) == 0 {
		return
	}

	memberID := data.Values[0]
	discordID := i.Member.User.ID

	// 1. find user
	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).First(&user).Error
	if err != nil {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	// 2. find member
	var member model.Member
	err = flow.DB.First(&member, memberID).Error
	if err != nil {
		respondError(s, i, "角色不存在")
		return
	}

	// 3. unbind member
	err = flow.DB.Model(&user).Association("Members").Delete(&member)
	if err != nil {
		log.Error().Err(err).Msgf("user unbind failed [%s@%s]", member.Name, member.Server)
		respondError(s, i, "无法解绑角色 内部错误")
		return
	}

	// 4. respond
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("✅ 解绑成功 %s@%s", member.Name, member.Server),
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		return
	}

	log.Info().Msgf("user unbind success [%s -> %s@%s]", discordID, member.Name, member.Server)
}

func handleHiddenSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	if len(data.Values) == 0 {
		return
	}

	memberID := data.Values[0]
	discordID := i.Member.User.ID

	// 1. find user
	var user model.User
	err := flow.DB.Where("discord_id = ?", discordID).First(&user).Error
	if err != nil {
		respondError(s, i, "目前没有绑定的角色 使用 `/bind` 绑定一个角色")
		return
	}

	// 2. find member
	var member model.Member
	err = flow.DB.First(&member, memberID).Error
	if err != nil {
		respondError(s, i, "角色不存在")
		return
	}

	// 3. verify user owns this member
	var existingCount int64
	flow.DB.Table("user_members").
		Where("user_id = ? AND member_id = ?", user.ID, member.ID).
		Count(&existingCount)

	if existingCount == 0 {
		respondError(s, i, "你没有绑定这个角色")
		return
	}

	// 4. toggle hidden status
	newHiddenStatus := !member.Hidden
	err = flow.DB.Model(&member).Update("hidden", newHiddenStatus).Error
	if err != nil {
		log.Error().Err(err).Msgf("toggle hidden status failed [%s@%s]", member.Name, member.Server)
		respondError(s, i, "无法修改隐藏状态 内部错误")
		return
	}

	// 5. respond
	statusText := "显示"
	if newHiddenStatus {
		statusText = "隐藏"
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("✅ 已将 %s@%s 设为 %s", member.Name, member.Server, statusText),
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		return
	}

	log.Info().Msgf("toggle hidden status success [%s -> %s@%s: %v]", discordID, member.Name, member.Server, newHiddenStatus)
}

func handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()

	switch data.CustomID {
	case "logs_modal":
		handleLogsModal(s, i)
	}
}

func respondSuccess(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("✅ %s", message),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ %s", message),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

func Stop() {
	if flow.Discord != nil {
		err := flow.Discord.Close()
		if err != nil {
			return
		}
		log.Info().Msg("discord bot session closed")
	}
}
