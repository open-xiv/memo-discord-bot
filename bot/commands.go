package bot

import "github.com/bwmarrin/discordgo"

// devCommandPerms is the DefaultMemberPermissions for dev-only commands. 0 hides
// the command from everyone except admins by default; an admin grants the dev
// role access once via Server Settings → Integrations → bot → command. A pointer
// to 0 is required — a plain int64(0) would be dropped by the omitempty tag and
// leave the command visible to all.
var devCommandPerms int64

var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        "bind",
		Description: "绑定游戏角色 (最多 3 个)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "name",
				Description:  "名称",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "server",
				Description:  "服务器",
				Required:     true,
				Autocomplete: true,
			},
		},
	},
	{
		Name:        "unbind",
		Description: "解绑游戏角色",
	},
	{
		Name:        "list",
		Description: "显示已绑定的游戏角色",
	},
	{
		Name:        "logs",
		Description: "使用 FFLogs 同步角色的战斗记录",
	},
	{
		Name:        "hide",
		Description: "设置已绑定角色：公开 / 不上榜 / 隐藏 (隐藏限一个角色)",
	},
	{
		Name:                     "set-hide",
		Description:              "管理：直接设置任意角色的隐私等级 (无需绑定)",
		DefaultMemberPermissions: &devCommandPerms,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "name",
				Description:  "角色名称",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "server",
				Description:  "服务器",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "level",
				Description: "隐私等级",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "公开", Value: 0},
					{Name: "不上榜", Value: 1},
					{Name: "隐藏", Value: 2},
				},
			},
		},
	},
}
