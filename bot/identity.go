package bot

import "github.com/bwmarrin/discordgo"

// Identity is the resolved actor behind an interaction. Discord ships the
// member's roles inside every guild interaction payload, so this is derived
// once at dispatch entry (see newCtx) rather than fetched — the analogue of
// memo-server's Authenticate middleware, minus the header/DB round-trip.
type Identity struct {
	roles map[string]bool
}

// IdentityOf reads the interacting guild member's roles. Member is nil for DM
// interactions; the bot registers commands guild-scoped so that path is inert,
// but the nil guard keeps Has/IsDev/IsSponsor safe regardless.
func IdentityOf(i *discordgo.InteractionCreate) Identity {
	if i.Member == nil {
		return Identity{}
	}
	roles := make(map[string]bool, len(i.Member.Roles))
	for _, r := range i.Member.Roles {
		roles[r] = true
	}
	return Identity{roles: roles}
}

func (id Identity) Has(roleID string) bool { return id.roles[roleID] }
func (id Identity) IsDev() bool            { return id.Has(RoleDevID) }
func (id Identity) IsSponsor() bool        { return id.Has(RoleSponsorID) }
