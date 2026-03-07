package handlers

import (
	"context"
	"log"
	"time"

	"blackonix/internal/core/agent"
	"blackonix/internal/core/state"
	"blackonix/internal/domain"
	"blackonix/internal/plugins"
	"blackonix/internal/ports"
	"blackonix/internal/repository"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	maxConcurrentWebhooks = 20
	webhookProcessTimeout = 60 * time.Second
)

type WebhookHandler struct {
	channelRepo      repository.ChannelRepository
	contactRepo      repository.ContactRepository
	sessionRepo      repository.SessionRepository
	messageRepo      repository.MessageRepository
	rocketChat       ports.RocketChatAPI
	llmClient        ports.LLMClient
	registry         *agent.ToolRegistry
	stateMachine     *state.Machine
	audioTranscriber *plugins.AudioTranscriberTool
	channels         map[domain.ChannelType]ports.MessagingChannel
	verifyToken      string
	systemPrompt     string
	sem              chan struct{}
}

type WebhookHandlerConfig struct {
	ChannelRepo      repository.ChannelRepository
	ContactRepo      repository.ContactRepository
	SessionRepo      repository.SessionRepository
	MessageRepo      repository.MessageRepository
	RocketChat       ports.RocketChatAPI
	LLMClient        ports.LLMClient
	Registry         *agent.ToolRegistry
	StateMachine     *state.Machine
	AudioTranscriber *plugins.AudioTranscriberTool
	Channels         map[domain.ChannelType]ports.MessagingChannel
	VerifyToken      string
	SystemPrompt     string
}

func NewWebhookHandler(cfg WebhookHandlerConfig) *WebhookHandler {
	return &WebhookHandler{
		channelRepo:      cfg.ChannelRepo,
		contactRepo:      cfg.ContactRepo,
		sessionRepo:      cfg.SessionRepo,
		messageRepo:      cfg.MessageRepo,
		rocketChat:       cfg.RocketChat,
		llmClient:        cfg.LLMClient,
		registry:         cfg.Registry,
		stateMachine:     cfg.StateMachine,
		audioTranscriber: cfg.AudioTranscriber,
		channels:         cfg.Channels,
		verifyToken:      cfg.VerifyToken,
		systemPrompt:     cfg.SystemPrompt,
		sem:              make(chan struct{}, maxConcurrentWebhooks),
	}
}

// VerifyWhatsAppWebhook handles Meta webhook verification (GET /webhook/whatsapp).
func (h *WebhookHandler) VerifyWhatsAppWebhook(c *fiber.Ctx) error {
	ch := h.channels[domain.ChannelWhatsApp]
	result, err := ch.VerifyWebhook(ports.VerifyRequest{
		Mode:        c.Query("hub.mode"),
		Token:       c.Query("hub.verify_token"),
		Challenge:   c.Query("hub.challenge"),
		VerifyToken: h.verifyToken,
	})
	if err != nil {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}
	return c.SendString(result)
}

// HandleWhatsAppWebhook processes WhatsApp messages (POST /webhook/whatsapp).
func (h *WebhookHandler) HandleWhatsAppWebhook(c *fiber.Ctx) error {
	return h.handleWebhook(c, domain.ChannelWhatsApp)
}

// HandleTelegramWebhook processes Telegram messages (POST /webhook/telegram/:token).
func (h *WebhookHandler) HandleTelegramWebhook(c *fiber.Ctx) error {
	urlToken := c.Params("token")
	if urlToken == "" {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	// Extract bot ID from token (format: <bot_id>:<hash>)
	botID := extractBotID(urlToken)
	channel, err := h.channelRepo.FindByExternalID(c.Context(), botID)
	if err != nil || channel.Credentials.Get("bot_token") != urlToken {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	ch, ok := h.channels[domain.ChannelTelegram]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "telegram not configured"})
	}

	body := c.Body()
	messages, err := ch.ParseWebhook(body)
	if err != nil {
		log.Printf("failed to parse telegram webhook: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	// Set the channel external ID on all parsed messages
	for i := range messages {
		messages[i].ChannelExternalID = botID
	}

	go func() {
		h.sem <- struct{}{}
		defer func() { <-h.sem }()
		h.processMessages(domain.ChannelTelegram, messages)
	}()

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) handleWebhook(c *fiber.Ctx, channelType domain.ChannelType) error {
	ch, ok := h.channels[channelType]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported channel"})
	}

	body := c.Body()
	messages, err := ch.ParseWebhook(body)
	if err != nil {
		log.Printf("failed to parse %s webhook: %v", channelType, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	go func() {
		h.sem <- struct{}{}
		defer func() { <-h.sem }()
		h.processMessages(channelType, messages)
	}()

	return c.SendStatus(fiber.StatusOK)
}

func (h *WebhookHandler) processMessages(channelType domain.ChannelType, messages []ports.NormalizedMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), webhookProcessTimeout)
	defer cancel()

	ch := h.channels[channelType]

	for _, msg := range messages {
		h.processNormalizedMessage(ctx, ch, msg)
	}
}

func (h *WebhookHandler) processNormalizedMessage(ctx context.Context, ch ports.MessagingChannel, msg ports.NormalizedMessage) {
	// 1. Find Channel by external ID
	channel, err := h.channelRepo.FindByExternalID(ctx, msg.ChannelExternalID)
	if err != nil {
		log.Printf("channel not found for %s message from %s: %v", msg.ChannelType, msg.From, err)
		return
	}

	// 2. Load/Create Contact
	contact, err := h.contactRepo.FindOrCreate(ctx, channel.TenantID, msg.From, msg.FromName)
	if err != nil {
		log.Printf("failed to find/create contact: %v", err)
		return
	}

	// 3. Load/Create Session
	session, err := h.sessionRepo.FindActiveByContact(ctx, channel.TenantID, contact.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			session = &domain.Session{
				TenantID:      channel.TenantID,
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

	// Extract text (transcribe audio if needed)
	textBody := msg.Text
	if textBody == "" && msg.Type != ports.MsgText {
		textBody = "[mídia recebida]"
	}
	if msg.Type == ports.MsgAudio && msg.MediaID != "" && h.audioTranscriber != nil {
		transcript, err := h.audioTranscriber.TranscribeFromChannel(ctx, ch, channel, msg.MediaID)
		if err != nil {
			log.Printf("failed to transcribe audio: %v", err)
			textBody = "[áudio recebido - falha na transcrição]"
		} else {
			textBody = transcript
		}
	}

	// Persist inbound message
	inboundMsg := &domain.Message{
		TenantID:  channel.TenantID,
		SessionID: session.ID,
		ContactID: contact.ID,
		Direction: domain.MessageDirectionInbound,
		Body:      textBody,
	}
	if err := h.messageRepo.Create(ctx, inboundMsg); err != nil {
		log.Printf("failed to save inbound message: %v", err)
	}

	// 4. If HUMAN -> forward to Rocket.Chat
	if h.stateMachine.IsHuman(session) {
		tenant := channel.Tenant
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

	// 5. If BOT -> process with Orchestrator
	contextRegistry := agent.NewToolRegistry()
	for _, tool := range h.registry.List() {
		contextRegistry.Register(tool)
	}
	contextRegistry.Register(plugins.NewTransferToHumanTool(
		h.stateMachine, h.rocketChat, session, &channel.Tenant, contact,
	))

	orchestrator := agent.NewOrchestrator(contextRegistry, h.llmClient, h.sessionRepo, h.systemPrompt)

	response, err := orchestrator.ProcessMessage(ctx, session, textBody)
	if err != nil {
		log.Printf("orchestrator error: %v", err)
		response = "Desculpe, estou com dificuldades no momento. Tente novamente em instantes."
	}

	// 6. Send response via channel
	if err := ch.SendResponse(ctx, channel, msg.From, ports.RichResponse{Text: response}); err != nil {
		log.Printf("failed to send %s response: %v", channel.Type, err)
	}

	// Persist outbound message
	outboundMsg := &domain.Message{
		TenantID:  channel.TenantID,
		SessionID: session.ID,
		ContactID: contact.ID,
		Direction: domain.MessageDirectionOutbound,
		Body:      response,
	}
	if err := h.messageRepo.Create(ctx, outboundMsg); err != nil {
		log.Printf("failed to save outbound message: %v", err)
	}
}

func extractBotID(token string) string {
	for i, c := range token {
		if c == ':' {
			return token[:i]
		}
	}
	return token
}
