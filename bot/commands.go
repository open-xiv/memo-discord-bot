package bot

import "github.com/bwmarrin/discordgo"

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
		Name:        "hidden",
		Description: "修改已绑定角色的隐藏状态",
	},
}
