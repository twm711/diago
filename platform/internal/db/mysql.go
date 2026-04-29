package db

import (
	"fmt"
	"time"

	"github.com/emiago/ai-call-center-platform/internal/call"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)

	if err := db.AutoMigrate(&call.Session{}, &call.Tenant{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return db, nil
}
