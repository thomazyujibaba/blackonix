package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")

	if botToken == "" || webhookURL == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_WEBHOOK_URL are required")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook?url=%s", botToken, webhookURL)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to set webhook: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if ok, _ := result["ok"].(bool); ok {
		fmt.Println("Telegram webhook set successfully!")
		fmt.Printf("URL: %s\n", webhookURL)
	} else {
		fmt.Printf("Failed to set webhook: %v\n", result)
		os.Exit(1)
	}
}
