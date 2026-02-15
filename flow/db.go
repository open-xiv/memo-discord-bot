package flow

import (
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
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = DB.AutoMigrate(&model.Member{}, &model.User{}, &model.LogsKey{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to auto migrate database")
	}
}
