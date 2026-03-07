package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	ctx := context.Background()

	// Create or find tenant
	var tenant domain.Tenant
	result := db.Where("name = ?", "Marina Presentes").First(&tenant)
	if result.Error != nil {
		tenant = domain.Tenant{
			Name: "Marina Presentes",
		}
		if err := db.Create(&tenant).Error; err != nil {
			log.Fatalf("Failed to create tenant: %v", err)
		}
		fmt.Printf("Created tenant: %s\n", tenant.ID)
	} else {
		fmt.Printf("Found existing tenant: %s\n", tenant.ID)
	}

	channelRepo := repository.NewChannelRepository(db)

	// Create WhatsApp channel if env vars provided
	wabaID := os.Getenv("WABA_ID")
	metaToken := os.Getenv("META_TOKEN")
	if wabaID != "" {
		_, err := channelRepo.FindByExternalID(ctx, wabaID)
		if err != nil {
			channel := domain.Channel{
				TenantID:   tenant.ID,
				Type:       domain.ChannelWhatsApp,
				ExternalID: wabaID,
				Credentials: domain.ChannelCredentials{
					"meta_token": metaToken,
					"waba_id":    wabaID,
				},
				Active: true,
			}
			if err := db.Create(&channel).Error; err != nil {
				log.Fatalf("Failed to create WhatsApp channel: %v", err)
			}
			fmt.Printf("Created WhatsApp channel: %s (WABA: %s)\n", channel.ID, wabaID)
		} else {
			fmt.Println("WhatsApp channel already exists")
		}
	}

	// Create Telegram channel if env vars provided
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken != "" {
		botID := botToken
		for i, c := range botToken {
			if c == ':' {
				botID = botToken[:i]
				break
			}
		}

		_, err := channelRepo.FindByExternalID(ctx, botID)
		if err != nil {
			channel := domain.Channel{
				TenantID:   tenant.ID,
				Type:       domain.ChannelTelegram,
				ExternalID: botID,
				Credentials: domain.ChannelCredentials{
					"bot_token": botToken,
				},
				Active: true,
			}
			if err := db.Create(&channel).Error; err != nil {
				log.Fatalf("Failed to create Telegram channel: %v", err)
			}
			fmt.Printf("Created Telegram channel: %s (Bot ID: %s)\n", channel.ID, botID)
		} else {
			fmt.Println("Telegram channel already exists")
		}
	}

	fmt.Println("Seed complete!")
}
