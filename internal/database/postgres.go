package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/raddigo/raddigo/internal/model"
)

// NewGORM opens a GORM connection to PostgreSQL and verifies connectivity.
func NewGORM(databaseURL string) (*gorm.DB, error) {
	gormLog := gormlogger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Migrate applies the schema for all models. It is safe to run on every startup.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.User{}, &model.Address{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
