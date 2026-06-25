package bot

// Chain composes middleware around a handler, applied left-to-right so the
// first listed runs outermost: Chain(h, A, B) == A(B(h)).
func Chain(h Handler, mw ...Middleware) Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// RequireRole gates a handler behind a Discord role, replying with denyMsg and
// short-circuiting when the actor lacks it — the gate analogue of memo-server's
// RequireSpecific. Roles ride along in the interaction payload, so no API call.
func RequireRole(roleID, denyMsg string) Middleware {
	return func(next Handler) Handler {
		return func(c *Ctx) {
			if !c.Identity.Has(roleID) {
				c.Error(denyMsg)
				return
			}
			next(c)
		}
	}
}
