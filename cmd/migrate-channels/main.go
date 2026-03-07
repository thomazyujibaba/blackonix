package main

import (
	"fmt"
	"log"

	"blackonix/internal/config"
	"blackonix/internal/domain"
	"blackonix/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := repository.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	type OldTenant struct {
		ID        string
		WabaID    string
		MetaToken string
	}

	var oldTenants []OldTenant
	if err := db.Raw("SELECT id, waba_id, meta_token FROM tenants WHERE waba_id IS NOT NULL AND waba_id != ''").Scan(&oldTenants).Error; err != nil {
		log.Printf("No tenants to migrate or columns already removed: %v", err)
		return
	}

	for _, ot := range oldTenants {
		channel := domain.Channel{
			TenantID:   ot.ID,
			Type:       domain.ChannelWhatsApp,
			ExternalID: ot.WabaID,
			Credentials: domain.ChannelCredentials{
				"meta_token": ot.MetaToken,
				"waba_id":    ot.WabaID,
			},
			Active: true,
		}

		if err := db.Create(&channel).Error; err != nil {
			log.Printf("Failed to create channel for tenant %s: %v", ot.ID, err)
			continue
		}
		fmt.Printf("Migrated tenant %s -> channel %s (WhatsApp, WABA: %s)\n", ot.ID, channel.ID, ot.WabaID)
	}

	if err := db.Exec("ALTER TABLE tenants DROP COLUMN IF EXISTS waba_id, DROP COLUMN IF EXISTS meta_token").Error; err != nil {
		log.Printf("Failed to drop old columns (may already be removed): %v", err)
	}

	fmt.Println("Migration complete!")
}
