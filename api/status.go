package api

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

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
	AsOf          time.Time        `json:"as_of"`
}

const (
	// healthRefreshInterval bounds how stale the cached check snapshot can be.
	// chosen so two refresh cycles fit inside the 30s freshness budget the
	// observability standard allows (memo-docs/standards/observability.md).
	healthRefreshInterval = 15 * time.Second
	// healthStaleThreshold makes StatusReady fail closed if the snapshot is
	// older than this — implies the background refresher has died.
	healthStaleThreshold = 30 * time.Second
	// healthProbeTimeout caps one refresh cycle's wait on any dependency.
	// independent of kubelet's probe timeout.
	healthProbeTimeout = 5 * time.Second
	// discordGatewayStaleAfter flags the gateway down if the last heartbeat
	// ack is older than this. Discord's heartbeat interval is ~41s, so 60s
	// allows one missed ack before we report unhealthy.
	discordGatewayStaleAfter = 60 * time.Second
	// criticalDepFailFastAfter triggers process exit when *every* critical
	// dep (DB + redis) has been failing every refresh tick for this long.
	// design intent: liveness probe cannot detect half-open conn pools
	// (process answers HTTP fine, but every query times out), so the
	// refresher itself acts as the kill switch — process exits non-zero,
	// kubelet's restartPolicy=Always recycles the pod, fresh pool gets a
	// clean network path. discord-gateway is excluded from the trigger
	// because websocket reconnects independently and false-positives here.
	criticalDepFailFastAfter = 90 * time.Second
)

type healthSnapshot struct {
	checks map[string]Check
	asOf   time.Time
}

var (
	cachedHealth atomic.Pointer[healthSnapshot]
	// firstCriticalFailure is the wall-clock time of the earliest tick in the
	// current run of consecutive "all critical deps down" results. nil whenever
	// any critical dep is healthy. drives the criticalDepFailFastAfter trigger.
	firstCriticalFailure atomic.Pointer[time.Time]
)

// StartHealthRefresher launches the background goroutine that refreshes the
// dep-check snapshot every healthRefreshInterval. callers should invoke this
// once at startup after dependencies are initialized; it primes the cache
// synchronously so the first StatusReady call doesn't see an empty snapshot.
//
// the kubelet readiness probe (StatusReady) reads only this cached value to
// stay O(1). fresh inline pinging on each probe caused 1s probe timeouts when
// deps live across the WG tunnel — see memo-docs/standards/observability.md.
func StartHealthRefresher(ctx context.Context) {
	refreshHealth(ctx)
	go func() {
		t := time.NewTicker(healthRefreshInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				refreshHealth(ctx)
			}
		}
	}()
}

func refreshHealth(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, healthProbeTimeout)
	defer cancel()
	checks := runChecks(ctx)
	cachedHealth.Store(&healthSnapshot{
		checks: checks,
		asOf:   time.Now().UTC(),
	})
	trackCriticalDepStreak(checks)
}

// trackCriticalDepStreak fails the process out when every critical dep has
// been failing on every refresh tick for criticalDepFailFastAfter. discord-
// gateway is intentionally excluded.
//
// rationale: a half-open WG / TLS conn pool can leave the process answering
// HTTP (so liveness stays green) while every query times out (so readiness
// stays red). nothing in k8s's two-probe model triggers a restart for that
// shape; we have to do it ourselves.
func trackCriticalDepStreak(checks map[string]Check) {
	criticalAllDown := !checks["database"].OK && !checks["redis"].OK
	if !criticalAllDown {
		firstCriticalFailure.Store(nil)
		return
	}
	if firstCriticalFailure.Load() == nil {
		now := time.Now()
		firstCriticalFailure.Store(&now)
		return
	}
	first := *firstCriticalFailure.Load()
	if time.Since(first) >= criticalDepFailFastAfter {
		log.Fatal().
			Dur("down_for", time.Since(first)).
			Msg("critical deps (database+redis) failing every refresh tick — exiting so kubelet can recycle")
	}
}

// Status is the human/monitor-facing endpoint; reads the cached snapshot.
// if the cache is somehow empty — e.g. called before StartHealthRefresher
// completed its priming write — it refreshes inline so the human reader
// doesn't see an empty payload.
func Status(c *gin.Context) {
	snap := cachedHealth.Load()
	if snap == nil {
		refreshHealth(c.Request.Context())
		snap = cachedHealth.Load()
	}

	overall := "ok"
	code := http.StatusOK
	for _, ch := range snap.checks {
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
		Checks:        snap.checks,
		AsOf:          snap.asOf,
	})
}

// StatusLive is the kubelet liveness probe: 200 iff the process can answer HTTP.
// failing liveness restarts the pod, so transient dep outages must never propagate here.
func StatusLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StatusReady is the kubelet readiness probe: 200 iff all critical dep checks pass.
// reads only the cached snapshot so it returns immediately regardless of dep RTT.
// fails closed (503) if the snapshot is older than healthStaleThreshold —
// implies the background refresher died.
func StatusReady(c *gin.Context) {
	snap := cachedHealth.Load()
	if snap == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "reason": "health snapshot not yet primed"})
		return
	}
	if time.Since(snap.asOf) > healthStaleThreshold {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "reason": "health snapshot stale"})
		return
	}
	for _, ch := range snap.checks {
		if !ch.OK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// runChecks returns the per-dependency check map for memo-discord-bot.
// the standard at memo-docs/standards/observability.md lists database, redis,
// and discord-gateway as critical-for-ready for this service.
func runChecks(ctx context.Context) map[string]Check {
	return map[string]Check{
		"database":        dbCheck(ctx),
		"redis":           redisCheck(ctx),
		"discord-gateway": discordGatewayCheck(),
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

// discordGatewayCheck confirms the bot's websocket session has received a
// heartbeat ack recently. LastHeartbeatAck advances on every Discord-server
// ack; if it stops updating, the connection is dead even when the goroutines
// haven't noticed yet. cheap because it's a local field read, not a network call.
func discordGatewayCheck() Check {
	if flow.Discord == nil {
		return Check{OK: false, Error: "discord session not initialized"}
	}
	ack := flow.Discord.LastHeartbeatAck
	if ack.IsZero() {
		return Check{OK: false, Error: "no heartbeat ack received yet"}
	}
	if time.Since(ack) > discordGatewayStaleAfter {
		return Check{OK: false, Error: "last heartbeat ack older than threshold"}
	}
	ms := flow.Discord.HeartbeatLatency().Milliseconds()
	return Check{OK: true, LatencyMs: &ms}
}
