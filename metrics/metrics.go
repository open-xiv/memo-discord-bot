package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal — bot's tiny gin HTTP server (/, /status, /metrics).
	// Mostly health-check noise from k8s; a counter is still useful as a
	// liveness signal.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "total number of HTTP requests received by the bot's status server",
		},
		[]string{"path", "method", "code"},
	)

	// InteractionsTotal — Discord-side interactions handled, by type and name.
	// Type is one of: app_command, autocomplete, component, modal.
	// Name is the slash-command name when applicable; empty otherwise.
	InteractionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_interactions_total",
			Help: "Discord interactions dispatched by the bot",
		},
		[]string{"type", "name"},
	)

	// SessionEvents — gateway lifecycle events. ready / disconnect / resume /
	// rate_limited. Spike on `disconnect` = bot lost gateway.
	SessionEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_session_events_total",
			Help: "Discord gateway session events",
		},
		[]string{"event"},
	)

	// WebhooksTotal — inbound webhook deliveries grouped by source path
	// (gha / github / ...), event name when known (e.g. release / push for
	// GitHub; "deploy" for our GHA notifier), and disposition status
	// (ok / unauthorized / duplicate / ignored / read_error / send_error /
	// bad_request / error).
	WebhooksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_webhooks_total",
			Help: "Inbound webhook deliveries handled by the bot",
		},
		[]string{"source", "event", "status"},
	)
)
