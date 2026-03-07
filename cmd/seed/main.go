package main

import (
	"fmt"
	"log"

	"blackonix/internal/config"
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

	result := db.Exec("UPDATE tenants SET waba_id = '2801340923547916' WHERE name = 'Marina Presentes'")
	if result.Error != nil {
		log.Fatalf("Failed to update tenant: %v", result.Error)
	}

	fmt.Printf("Updated %d rows\n", result.RowsAffected)
}
