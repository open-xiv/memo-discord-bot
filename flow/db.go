package flow

import (
	"context"
	"os"
	"time"

	"github.com/open-xiv/memo-discord-bot/model"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=fight_memo port=5432 sslmode=disable TimeZone=UTC"
		log.Warn().Msgf("DATABASE_URL not set, using %v", dsn)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect database")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get database instance")
	}

	sqlDB.SetMaxOpenConns(15)
	sqlDB.SetMaxIdleConns(10)
	// short-ish lifetime so half-open conns through the WG tunnel cycle
	// out instead of getting stuck for an hour. ~5 min is a sweet spot
	// between pool churn cost and stale-conn risk for our query rate.
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)

	// explicit ping — gorm.Open is lazy and AutoMigrate has been observed
	// to no-op (cached schema) without ever opening a real connection.
	// without this, the bot could start up "successfully" with an
	// unreachable DB and only surface the failure via readiness probes,
	// which never trigger a restart.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Msg("database ping failed at startup")
	}

	if err := DB.AutoMigrate(&model.Member{}, &model.User{}, &model.LogsKey{}); err != nil {
		log.Fatal().Err(err).Msg("failed to auto migrate database")
	}
}

// StartKeepalive keeps one DB connection warm in the pool with a SELECT-1-
// shaped ping every 30s. memo-docs/standards/observability.md "Dependency
// pool warmth" — DO NOT REMOVE without reading that section first.
//
// memo-discord-bot is event-driven: Discord interactions arrive in bursts,
// most of the day the pool sits idle. SetConnMaxIdleTime(2m) closes idle
// conns, and Cloud SQL / NAT conntrack also reap idle 5-tuples on their
// own ~5min timer. Without this goroutine every /status refresh tick was
// paying a full cold handshake (TCP + TLS for sslmode=require + PG STARTUP)
// over WG mesh0 → droplet-hk NAT → VPC peering → Cloud SQL, which was
// timing out the 5s probe budget ~50% of the time and showing up in
// Grafana as a flapping database check. 30s was picked one order of
// magnitude under the 5min idle timeouts so a single missed tick still
// leaves margin.
//
// Self-heal: this also catches the "pool full of half-open conns" failure
// mode, where pgx holds connections it believes are established but whose
// TCP path is dead (typically: new pod's first NAT conntrack entries get
// wedged after a rollout, every PingContext writes to a socket that never
// gets a reply). The keepalive ping itself can't recover from that — it
// just observes it. After failuresBeforeFatal consecutive misses (~90s of
// confirmed-broken DB) we Fatal so the kubelet restarts the pod, which
// rebuilds NAT state and the pool from scratch. This is the same recovery
// path manual `kubectl rollout restart` exercises today.
func StartKeepalive(parent context.Context) {
	const failuresBeforeFatal = 3

	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		consecutive := 0
		for {
			select {
			case <-parent.Done():
				return
			case <-t.C:
				sqlDB, err := DB.DB()
				if err != nil {
					log.Warn().Str("event", "db.keepalive_failed").Err(err).Msg("keepalive: failed to get sql.DB")
					continue
				}
				ctx, cancel := context.WithTimeout(parent, 5*time.Second)
				err = sqlDB.PingContext(ctx)
				cancel()
				if err == nil {
					consecutive = 0
					continue
				}
				consecutive++
				log.Warn().Str("event", "db.keepalive_failed").Int("consecutive", consecutive).Err(err).Msg("keepalive ping failed")
				if consecutive >= failuresBeforeFatal {
					log.Fatal().Str("event", "db.keepalive_fatal").Int("consecutive", consecutive).Msg("db keepalive failed repeatedly; exiting to force pod restart")
				}
			}
		}
	}()
}
