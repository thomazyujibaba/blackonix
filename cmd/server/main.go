package main

import (
	"fmt"
	"log"

	"blackonix/internal/adapters/llm"
	"blackonix/internal/adapters/meta"
	"blackonix/internal/adapters/rocketchat"
	"blackonix/internal/config"
	"blackonix/internal/core/agent"
	"blackonix/internal/core/state"
	"blackonix/internal/handlers"
	"blackonix/internal/plugins"
	"blackonix/internal/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const systemPrompt = `Você é o assistente virtual da loja. Seja educado, objetivo e útil.
Você pode consultar o estoque de produtos e transferir o cliente para um atendente humano quando necessário.
Responda sempre em português brasileiro.`

func main() {
	// 1. Configuração
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Banco de dados
	db, err := repository.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 3. Repositories
	tenantRepo := repository.NewTenantRepository(db)
	contactRepo := repository.NewContactRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// 4. Adapters (Ports implementations)
	metaAPI := meta.NewMetaClient()
	rocketChatAPI := rocketchat.NewRocketChatClient()
	llmClient := llm.NewOpenAIClient(cfg.LLMApiKey, cfg.LLMModel)

	// 5. Core
	stateMachine := state.NewMachine(sessionRepo)

	// 6. Tool Registry (plugins globais)
	registry := agent.NewToolRegistry()
	registry.Register(plugins.NewCheckStockTool())
	// TransferToHumanTool é registrado por request (depende da session)

	// 7. Fiber App
	app := fiber.New(fiber.Config{
		AppName: "BlackOnix Agentic Middleware",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	// 8. Webhook Handler com DI
	webhookHandler := handlers.NewWebhookHandler(handlers.WebhookHandlerConfig{
		TenantRepo:   tenantRepo,
		ContactRepo:  contactRepo,
		SessionRepo:  sessionRepo,
		MessageRepo:  messageRepo,
		MetaAPI:      metaAPI,
		RocketChat:   rocketChatAPI,
		LLMClient:    llmClient,
		Registry:     registry,
		StateMachine: stateMachine,
		VerifyToken:  cfg.MetaVerifyToken,
		SystemPrompt: systemPrompt,
	})

	// 9. Rotas
	app.Get("/webhook", webhookHandler.VerifyWebhook)
	app.Post("/webhook", webhookHandler.HandleWebhook)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "blackonix",
		})
	})

	// 10. Start
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("BlackOnix starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
