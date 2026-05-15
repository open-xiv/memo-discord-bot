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
