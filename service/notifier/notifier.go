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

// Embed colors — Tailwind v4 macaron palette (the -300 row) so the
// stripe reads as a pastel signal rather than a saturated alarm.
// Picked to be readable on Discord's dark theme without screaming.
const (
	ColorSuccess    = 0x86EFAC // green-300  — successful deploy
	ColorFailure    = 0xFDA4AF // rose-300   — failed deploy / build
	ColorInProgress = 0xFCD34D // amber-300  — running, not yet final
	ColorRollback   = 0xC4B5FD // violet-300 — rollback in flight or done
	ColorInfo       = 0x93C5FD // blue-300   — neutral notice
	ColorWarn       = 0xFCD34D // amber-300  — alias for warnings
)

// IconBaseURL hosts the macaron-tinted status PNGs in the assets repo
// (common/icons/deploy/) → cached on Cloudflare via assets.sumemo.dev.
// Add new entries when more SVGs land in common/icons/deploy/.
const IconBaseURL = "https://assets.sumemo.dev/icons/deploy"

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
