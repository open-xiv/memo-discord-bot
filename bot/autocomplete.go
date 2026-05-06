package bot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
)

var servers = []string{
	"陆行鸟", "莫古力", "猫小胖", "豆豆柴",
	"红玉海", "神意之地", "拉诺西亚", "幻影群岛",
	"萌芽池", "宇宙和音", "沃仙曦染", "晨曦王座",
	"白银乡", "白金幻象", "神拳痕", "潮风亭",
	"旅人栈桥", "拂晓之间", "龙巢神殿", "梦羽宝境",
	"紫水栈桥", "延夏", "静语庄园", "摩杜纳",
	"海猫茶屋", "柔风海湾", "琥珀原",
	"水晶塔", "银泪湖", "太阳海岸", "伊修加德", "红茶川",
}

func handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Focused {
			focusedOption = opt
			break
		}
	}

	if focusedOption == nil {
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice

	switch focusedOption.Name {

	case "name":
		query := focusedOption.StringValue()
		if query == "" {
			break
		}

		var members []model.Member
		err := flow.DB.
			Where("LOWER(name) LIKE ?", "%"+strings.ToLower(query)+"%").
			Limit(25).
			Find(&members).Error

		if err != nil {
			log.Error().Err(err).Msgf("autocomplete query [%s] failed (%s)", query, focusedOption.Name)
			return
		}

		choices = make([]*discordgo.ApplicationCommandOptionChoice, len(members))
		for i, member := range members {
			choices[i] = &discordgo.ApplicationCommandOptionChoice{
				Name:  member.Name,
				Value: member.Name,
			}
		}

	case "server":
		query := strings.ToLower(focusedOption.StringValue())

		for _, server := range servers {
			if query == "" || strings.Contains(strings.ToLower(server), query) || strings.Contains(server, query) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  server,
					Value: server,
				})

				if len(choices) >= 25 {
					break
				}
			}
		}
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("autocomplete response failed")
	}
}
