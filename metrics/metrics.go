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
)
