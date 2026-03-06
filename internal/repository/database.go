package repository

import (
	"blackonix/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&domain.Tenant{},
		&domain.Contact{},
		&domain.Session{},
		&domain.Message{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
