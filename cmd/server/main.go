package main

import (
	"fmt"
	"log"

	"blackonix/internal/adapters/llm"
	"blackonix/internal/adapters/meta"
	"blackonix/internal/adapters/rocketchat"
	"blackonix/internal/adapters/telegram"
	"blackonix/internal/config"
	"blackonix/internal/core/agent"
	"blackonix/internal/core/state"
	"blackonix/internal/domain"
	"blackonix/internal/handlers"
	"blackonix/internal/plugins"
	"blackonix/internal/ports"
	"blackonix/internal/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const systemPrompt = `Você é o assistente virtual da loja. Seja educado, objetivo e útil.
Você pode transcrever áudios e transferir o cliente para um atendente humano quando necessário.
Se o cliente enviar um áudio, você receberá a transcrição automaticamente.
Responda sempre em português brasileiro.`

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := repository.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Repositories
	channelRepo := repository.NewChannelRepository(db)
	contactRepo := repository.NewContactRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Channel adapters
	channelAdapters := map[domain.ChannelType]ports.MessagingChannel{
		domain.ChannelWhatsApp:  meta.NewMetaChannel(),
		domain.ChannelTelegram: telegram.NewTelegramChannel(),
	}

	rocketChatAPI := rocketchat.NewRocketChatClient()
	llmClient := llm.NewOpenAIClient(cfg.LLMApiKey, cfg.LLMModel)

	// Core
	stateMachine := state.NewMachine(sessionRepo)
	registry := agent.NewToolRegistry()
	audioTranscriber := plugins.NewAudioTranscriberTool(cfg.LLMApiKey)

	// Fiber App
	app := fiber.New(fiber.Config{
		AppName: "BlackOnix Agentic Middleware",
	})
	app.Use(logger.New())
	app.Use(recover.New())

	// Webhook Handler
	webhookHandler := handlers.NewWebhookHandler(handlers.WebhookHandlerConfig{
		ChannelRepo:      channelRepo,
		ContactRepo:      contactRepo,
		SessionRepo:      sessionRepo,
		MessageRepo:      messageRepo,
		RocketChat:       rocketChatAPI,
		LLMClient:        llmClient,
		Registry:         registry,
		StateMachine:     stateMachine,
		AudioTranscriber: audioTranscriber,
		Channels:         channelAdapters,
		VerifyToken:      cfg.MetaVerifyToken,
		SystemPrompt:     systemPrompt,
	})

	// Routes
	app.Get("/webhook/whatsapp", webhookHandler.VerifyWhatsAppWebhook)
	app.Post("/webhook/whatsapp", webhookHandler.HandleWhatsAppWebhook)

	// Telegram
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "blackonix",
		})
	})

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("BlackOnix starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
