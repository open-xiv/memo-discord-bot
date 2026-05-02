// Package notifier centralizes outbound Discord pushes (embeds + plain
// messages). All webhook handlers go through here so we can swap in
// rate-limit / retry / channel-routing logic without touching individual
// handlers.
package notifier

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/open-xiv/memo-discord-bot/flow"
)

// Embed colors picked to read well in both Discord light + dark themes.
const (
	ColorSuccess = 0x57F287 // green
	ColorFailure = 0xED4245 // red
	ColorInfo    = 0x5865F2 // blurple
	ColorWarn    = 0xFEE75C // yellow
)

// SendEmbed posts a single embed to a channel and returns the resulting
// message (so caller can edit / reply later if needed).
func SendEmbed(channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if flow.Discord == nil {
		return nil, fmt.Errorf("notifier: discord session not initialized")
	}
	return flow.Discord.ChannelMessageSendEmbed(channelID, embed)
}

// SendText is the plain-text fallback. Used for tiny ephemeral signals
// (e.g. ping confirmation) where an embed is overkill.
func SendText(channelID, content string) (*discordgo.Message, error) {
	if flow.Discord == nil {
		return nil, fmt.Errorf("notifier: discord session not initialized")
	}
	return flow.Discord.ChannelMessageSend(channelID, content)
}
