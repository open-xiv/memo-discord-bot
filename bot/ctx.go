package bot

import "github.com/bwmarrin/discordgo"

// Ctx carries the session, the interaction, and the resolved Identity through
// the whole handler chain — the discordgo equivalent of gin's *gin.Context.
// Identity is attached once at dispatch entry so handlers never re-parse roles.
type Ctx struct {
	S        *discordgo.Session
	I        *discordgo.InteractionCreate
	Identity Identity
}

// Handler is the per-interaction handler signature; Middleware wraps one.
type (
	Handler    = func(*Ctx)
	Middleware = func(Handler) Handler
)

func newCtx(s *discordgo.Session, i *discordgo.InteractionCreate) *Ctx {
	return &Ctx{S: s, I: i, Identity: IdentityOf(i)}
}

// DiscordID returns the invoking user's ID, from Member (guild) or User (DM).
func (c *Ctx) DiscordID() string {
	if c.I.Member != nil && c.I.Member.User != nil {
		return c.I.Member.User.ID
	}
	if c.I.User != nil {
		return c.I.User.ID
	}
	return ""
}

func (c *Ctx) Error(message string)   { respondError(c.S, c.I, message) }
func (c *Ctx) Success(message string) { respondSuccess(c.S, c.I, message) }
