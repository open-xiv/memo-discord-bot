package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/open-xiv/memo-discord-bot/buildinfo"
	"github.com/open-xiv/memo-discord-bot/flow"
)

type Check struct {
	OK        bool   `json:"ok"`
	LatencyMs *int64 `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type StatusResponse struct {
	Service       string           `json:"service"`
	Version       string           `json:"version"`
	Build         string           `json:"build"`
	Env           string           `json:"env"`
	StartedAt     time.Time        `json:"started_at"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Status        string           `json:"status"`
	Checks        map[string]Check `json:"checks"`
}

// Status is the human/monitor-facing endpoint; runs dep probes inline (not cached, since calls are infrequent).
func Status(c *gin.Context) {
	checks := runChecks(c.Request.Context())

	overall := "ok"
	code := http.StatusOK
	for _, ch := range checks {
		if !ch.OK {
			overall = "down"
			code = http.StatusServiceUnavailable
			break
		}
	}

	c.JSON(code, StatusResponse{
		Service:       buildinfo.Service,
		Version:       buildinfo.Version,
		Build:         buildinfo.Build,
		Env:           buildinfo.Env,
		StartedAt:     buildinfo.StartedAt,
		UptimeSeconds: int64(time.Since(buildinfo.StartedAt).Seconds()),
		Status:        overall,
		Checks:        checks,
	})
}

// StatusLive is the kubelet liveness probe: 200 iff the process can answer HTTP.
// failing liveness restarts the pod, so transient dep outages must never propagate here.
func StatusLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StatusReady is the kubelet readiness probe: 200 iff all critical dep checks pass, otherwise 503.
// failing readiness pulls the pod from service endpoints but doesn't restart it.
func StatusReady(c *gin.Context) {
	checks := runChecks(c.Request.Context())
	for _, ch := range checks {
		if !ch.OK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func runChecks(ctx context.Context) map[string]Check {
	return map[string]Check{
		"database": dbCheck(ctx),
		"redis":    redisCheck(ctx),
	}
}

func dbCheck(ctx context.Context) Check {
	start := time.Now()
	sqlDB, err := flow.DB.DB()
	if err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	ms := time.Since(start).Milliseconds()
	return Check{OK: true, LatencyMs: &ms}
}

func redisCheck(ctx context.Context) Check {
	start := time.Now()
	if err := flow.Redis.Ping(ctx).Err(); err != nil {
		return Check{OK: false, Error: err.Error()}
	}
	ms := time.Since(start).Milliseconds()
	return Check{OK: true, LatencyMs: &ms}
}
