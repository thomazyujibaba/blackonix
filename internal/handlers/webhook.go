package handlers

import (
	"context"
	"log"

	"blackonix/internal/core/agent"
	"blackonix/internal/core/state"
	"blackonix/internal/domain"
	"blackonix/internal/plugins"
	"blackonix/internal/ports"
	"blackonix/internal/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	tenantRepo       repository.TenantRepository
	contactRepo      repository.ContactRepository
	sessionRepo      repository.SessionRepository
	messageRepo      repository.MessageRepository
	metaAPI          ports.MetaAPI
	rocketChat       ports.RocketChatAPI
	llmClient        ports.LLMClient
	registry         *agent.ToolRegistry
	stateMachine     *state.Machine
	audioTranscriber *plugins.AudioTranscriberTool
	verifyToken      string
	systemPrompt     string
}

type WebhookHandlerConfig struct {
	TenantRepo       repository.TenantRepository
	ContactRepo      repository.ContactRepository
	SessionRepo      repository.SessionRepository
	MessageRepo      repository.MessageRepository
	MetaAPI          ports.MetaAPI
	RocketChat       ports.RocketChatAPI
	LLMClient        ports.LLMClient
	Registry         *agent.ToolRegistry
	StateMachine     *state.Machine
	AudioTranscriber *plugins.AudioTranscriberTool
	VerifyToken      string
	SystemPrompt     string
}

func NewWebhookHandler(cfg WebhookHandlerConfig) *WebhookHandler {
	return &WebhookHandler{
		tenantRepo:       cfg.TenantRepo,
		contactRepo:      cfg.ContactRepo,
		sessionRepo:      cfg.SessionRepo,
		messageRepo:      cfg.MessageRepo,
		metaAPI:          cfg.MetaAPI,
		rocketChat:       cfg.RocketChat,
		llmClient:        cfg.LLMClient,
		registry:         cfg.Registry,
		stateMachine:     cfg.StateMachine,
		audioTranscriber: cfg.AudioTranscriber,
		verifyToken:      cfg.VerifyToken,
		systemPrompt:     cfg.SystemPrompt,
	}
}

// VerifyWebhook responde ao desafio de verificação da Meta (GET).
func (h *WebhookHandler) VerifyWebhook(c *fiber.Ctx) error {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	result, err := h.metaAPI.VerifyWebhook(mode, token, challenge, h.verifyToken)
	if err != nil {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	return c.SendString(result)
}

// HandleWebhook processa mensagens recebidas da Meta (POST).
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	var payload MetaWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		log.Printf("failed to parse webhook payload: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Responde 200 imediatamente para a Meta não reenviar
	// Processa em background
	go h.processPayload(payload)

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) processPayload(payload MetaWebhookPayload) {
	ctx := context.Background()

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			wabaID := entry.ID

			// 1. Valida Tenant pelo WABA ID
			tenant, err := h.tenantRepo.FindByWabaID(ctx, wabaID)
			if err != nil {
				log.Printf("tenant not found for WABA %s: %v", wabaID, err)
				continue
			}

			for _, msg := range change.Value.Messages {
				h.processMessage(ctx, tenant, change.Value.Metadata.PhoneNumberID, msg)
			}
		}
	}
}

func (h *WebhookHandler) processMessage(ctx context.Context, tenant *domain.Tenant, phoneNumberID string, msg MetaMessage) {
	// 2. Carrega/Cria Contact
	contact, err := h.contactRepo.FindOrCreate(ctx, tenant.ID, msg.From, msg.From)
	if err != nil {
		log.Printf("failed to find/create contact: %v", err)
		return
	}

	// 3. Carrega/Cria Session
	session, err := h.sessionRepo.FindActiveByContact(ctx, tenant.ID, contact.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			session = &domain.Session{
				TenantID:      tenant.ID,
				ContactID:     contact.ID,
				State:         domain.SessionStateBot,
				ContextMemory: domain.ContextMemory{},
			}
			if err := h.sessionRepo.Create(ctx, session); err != nil {
				log.Printf("failed to create session: %v", err)
				return
			}
		} else {
			log.Printf("failed to find session: %v", err)
			return
		}
	}

	// Extrai o texto (transcreve áudio se necessário)
	textBody := extractTextBody(msg)
	if msg.Type == "audio" && msg.Audio != nil && h.audioTranscriber != nil {
		transcript, err := h.audioTranscriber.Execute(ctx, map[string]interface{}{
			"media_id":   msg.Audio.ID,
			"meta_token": tenant.MetaToken,
		})
		if err != nil {
			log.Printf("failed to transcribe audio: %v", err)
			textBody = "[áudio recebido - falha na transcrição]"
		} else {
			textBody = transcript
		}
	}

	// Persiste mensagem inbound
	inboundMsg := &domain.Message{
		TenantID:  tenant.ID,
		SessionID: session.ID,
		ContactID: contact.ID,
		Direction: domain.MessageDirectionInbound,
		Body:      textBody,
	}
	if err := h.messageRepo.Create(ctx, inboundMsg); err != nil {
		log.Printf("failed to save inbound message: %v", err)
	}

	// 4. Se HUMAN -> envia para Rocket.Chat
	if h.stateMachine.IsHuman(session) {
		if err := h.rocketChat.SendMessage(
			ctx,
			tenant.RocketChatURL,
			tenant.RocketChatToken,
			session.ActiveDepartment,
			contact.Name,
			contact.PhoneNumber,
			textBody,
		); err != nil {
			log.Printf("failed to forward to rocketchat: %v", err)
		}
		return
	}

	// 5. Se BOT -> processa com o Orchestrator
	// Registra tools contextuais (que dependem da sessão atual)
	contextRegistry := agent.NewToolRegistry()
	for _, tool := range h.registry.List() {
		contextRegistry.Register(tool)
	}
	contextRegistry.Register(plugins.NewTransferToHumanTool(
		h.stateMachine, h.rocketChat, session, tenant, contact,
	))

	orchestrator := agent.NewOrchestrator(contextRegistry, h.llmClient, h.sessionRepo, h.systemPrompt)

	response, err := orchestrator.ProcessMessage(ctx, session, textBody)
	if err != nil {
		log.Printf("orchestrator error: %v", err)
		response = "Desculpe, estou com dificuldades no momento. Tente novamente em instantes."
	}

	// 6. Envia resposta via WhatsApp
	if err := h.metaAPI.SendTextMessage(ctx, tenant.MetaToken, phoneNumberID, contact.PhoneNumber, response); err != nil {
		log.Printf("failed to send whatsapp response: %v", err)
	}

	// Persiste mensagem outbound
	outboundMsg := &domain.Message{
		TenantID:  tenant.ID,
		SessionID: session.ID,
		ContactID: contact.ID,
		Direction: domain.MessageDirectionOutbound,
		Body:      response,
	}
	if err := h.messageRepo.Create(ctx, outboundMsg); err != nil {
		log.Printf("failed to save outbound message: %v", err)
	}
}

func extractTextBody(msg MetaMessage) string {
	if msg.Text != nil {
		return msg.Text.Body
	}
	return "[mensagem não-textual recebida]"
}
