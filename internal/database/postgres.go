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
	// PostGIS powers the spatial point-in-polygon Partner search.
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS postgis`).Error; err != nil {
		return fmt.Errorf("enable postgis: %w", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Partner{},
		&model.Address{},
		&model.PolygonPoint{},
		&model.Rating{},
		&model.Booking{},
		&model.BookingImage{},
		&model.BookingCompletionImage{},
		&model.BookingStatusLog{},
		&model.PartnerSubscription{},
		&model.SubscriptionTransaction{},
		&model.PromoCode{},
		&model.Notification{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Migrate any legacy ragman store addresses into the merged addresses table,
	// then drop the old table. Idempotent: skipped once the table is gone.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF to_regclass('public.ragman_store_addresses') IS NOT NULL THEN
				INSERT INTO addresses (
					id, type, partner_id, address1, address2, street, city, state,
					country, pincode, latitude, longitude, created_at, updated_at
				)
				SELECT id, 'partner_store', ragman_id, address1, address2, street,
				       city, state, country, pincode, latitude, longitude,
				       created_at, updated_at
				FROM ragman_store_addresses
				ON CONFLICT (id) DO NOTHING;
				DROP TABLE ragman_store_addresses;
			END IF;
		END $$;
	`).Error; err != nil {
		return fmt.Errorf("migrate legacy store addresses: %w", err)
	}

	// Migrate any pre-existing single scrap_image value into the new
	// booking_images table, then drop the deprecated column. Idempotent:
	// skipped once the column is gone.
	if err := db.Exec(`
		DO $$
		BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'bookings' AND column_name = 'scrap_image'
			) THEN
				INSERT INTO booking_images (id, booking_id, sequence, url, created_at)
				SELECT gen_random_uuid(), id, 0, scrap_image, created_at
				FROM bookings
				WHERE scrap_image IS NOT NULL AND scrap_image <> '';

				ALTER TABLE bookings DROP COLUMN scrap_image;
			END IF;
		END $$;
	`).Error; err != nil {
		return fmt.Errorf("migrate legacy scrap_image column: %w", err)
	}

	// A geography(Polygon) column plus a GiST index makes ST_Covers point-in-
	// polygon lookups use a bounding-box index scan instead of a full table scan.
	spatial := []string{
		`ALTER TABLE partners ADD COLUMN IF NOT EXISTS service_area geography(Polygon,4326)`,
		`CREATE INDEX IF NOT EXISTS idx_partners_service_area ON partners USING GIST (service_area)`,
	}
	for _, stmt := range spatial {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("spatial migrate: %w", err)
		}
	}

	// Enforce that a partner can have at most one accepted booking per slot, so
	// accepting a booking disables that slot for any competing request.
	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_accepted_slot
		 ON bookings (partner_id, slot_date, slot_start_time)
		 WHERE status = 'accepted'`,
	).Error; err != nil {
		return fmt.Errorf("booking slot index: %w", err)
	}
	return nil
}
