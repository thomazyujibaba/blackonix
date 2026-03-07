# Telegram Adapter + Channel Abstraction — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Telegram as a second messaging channel by introducing a channel abstraction layer that decouples the core pipeline from any specific platform.

**Architecture:** New `Channel` domain model with JSON credentials replaces per-platform fields on `Tenant`. A `MessagingChannel` interface normalizes messages across platforms. The webhook handler becomes generic, receiving parsed `NormalizedMessage` structs. Meta and Telegram adapters both implement `MessagingChannel`.

**Tech Stack:** Go 1.25, Fiber v2, GORM (PostgreSQL), Telegram Bot API (direct HTTP, no library)

---

### Task 1: Create `Channel` Domain Model

**Files:**
- Create: `internal/domain/channel.go`

**Step 1: Create the Channel model**

```go
package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type ChannelType string

const (
	ChannelWhatsApp ChannelType = "whatsapp"
	ChannelTelegram ChannelType = "telegram"
)

// ChannelCredentials stores platform-specific credentials as JSON in the database.
type ChannelCredentials map[string]string

func (cc ChannelCredentials) Value() (driver.Value, error) {
	return json.Marshal(cc)
}

func (cc *ChannelCredentials) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan ChannelCredentials: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, cc)
}

// Get returns a credential value or empty string if not found.
func (cc ChannelCredentials) Get(key string) string {
	if cc == nil {
		return ""
	}
	return cc[key]
}

type Channel struct {
	ID          string             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    string             `gorm:"type:uuid;not null;index"`
	Type        ChannelType        `gorm:"type:varchar(20);not null"`
	Credentials ChannelCredentials `gorm:"type:jsonb;not null;default:'{}'"`
	ExternalID  string             `gorm:"uniqueIndex;not null"` // WABA ID or Telegram Bot ID
	Active      bool               `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tenant Tenant `gorm:"foreignKey:TenantID"`
}
```

**Step 2: Register Channel in GORM AutoMigrate**

In `internal/repository/database.go`, add `&domain.Channel{}` to the AutoMigrate call:

```go
if err := db.AutoMigrate(
	&domain.Tenant{},
	&domain.Channel{},   // <-- add this
	&domain.Contact{},
	&domain.Session{},
	&domain.Message{},
	&domain.User{},
); err != nil {
```

**Step 3: Add ChannelID to Contact**

In `internal/domain/contact.go`, add:

```go
type Contact struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    string `gorm:"type:uuid;not null;index"`
	ChannelID   string `gorm:"type:uuid;index"`            // <-- add this
	PhoneNumber string `gorm:"not null;index"`
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Tenant  Tenant  `gorm:"foreignKey:TenantID"`
	Channel Channel `gorm:"foreignKey:ChannelID"`          // <-- add this
}
```

**Step 4: Build and verify**

Run: `go build ./...`
Expected: Clean build, no errors.

**Step 5: Commit**

```bash
git add internal/domain/channel.go internal/domain/contact.go internal/repository/database.go
git commit -m "feat: add Channel domain model and ChannelID to Contact"
```

---

### Task 2: Create `ChannelRepository`

**Files:**
- Modify: `internal/repository/interfaces.go`
- Create: `internal/repository/channel_repo.go`

**Step 1: Add ChannelRepository interface**

In `internal/repository/interfaces.go`, add:

```go
type ChannelRepository interface {
	FindByExternalID(ctx context.Context, externalID string) (*domain.Channel, error)
	FindByTenantAndType(ctx context.Context, tenantID string, channelType domain.ChannelType) (*domain.Channel, error)
	Create(ctx context.Context, channel *domain.Channel) error
	Update(ctx context.Context, channel *domain.Channel) error
	List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Channel], error)
}
```

**Step 2: Create channel_repo.go**

```go
package repository

import (
	"context"

	"blackonix/internal/domain"
	"gorm.io/gorm"
)

type channelRepo struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) ChannelRepository {
	return &channelRepo{db: db}
}

func (r *channelRepo) FindByExternalID(ctx context.Context, externalID string) (*domain.Channel, error) {
	var channel domain.Channel
	if err := r.db.WithContext(ctx).Preload("Tenant").Where("external_id = ? AND active = true", externalID).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepo) FindByTenantAndType(ctx context.Context, tenantID string, channelType domain.ChannelType) (*domain.Channel, error) {
	var channel domain.Channel
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND type = ? AND active = true", tenantID, channelType).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *channelRepo) Create(ctx context.Context, channel *domain.Channel) error {
	return r.db.WithContext(ctx).Create(channel).Error
}

func (r *channelRepo) Update(ctx context.Context, channel *domain.Channel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

func (r *channelRepo) List(ctx context.Context, tenantID string, params PaginationParams) (*PaginatedResult[domain.Channel], error) {
	var channels []domain.Channel
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Channel{})
	if tenantID != "" {
		q = q.Where("tenant_id = ?", tenantID)
	}

	q.Count(&total)

	offset := (params.Page - 1) * params.Limit
	if err := q.Preload("Tenant").Order("created_at DESC").Offset(offset).Limit(params.Limit).Find(&channels).Error; err != nil {
		return nil, err
	}

	return &PaginatedResult[domain.Channel]{
		Data: channels, Total: total, Page: params.Page, Limit: params.Limit,
	}, nil
}
```

**Step 3: Build and verify**

Run: `go build ./...`
Expected: Clean build.

**Step 4: Commit**

```bash
git add internal/repository/interfaces.go internal/repository/channel_repo.go
git commit -m "feat: add ChannelRepository interface and GORM implementation"
```

---

### Task 3: Create `MessagingChannel` Interface and Normalized Types

**Files:**
- Create: `internal/ports/messaging.go`

**Step 1: Create the messaging port**

```go
package ports

import (
	"blackonix/internal/domain"
	"context"
	"time"
)

type MessageType string

const (
	MsgText  MessageType = "text"
	MsgAudio MessageType = "audio"
	MsgPhoto MessageType = "photo"
	MsgVideo MessageType = "video"
)

// NormalizedMessage is a channel-agnostic representation of an incoming message.
type NormalizedMessage struct {
	ExternalID    string
	ChannelType   domain.ChannelType
	From          string // Phone number (WhatsApp) or chat ID (Telegram)
	FromName      string
	Type          MessageType
	Text          string
	MediaID       string
	MediaMimeType string
	Timestamp     time.Time
}

// RichResponse supports text with optional inline buttons and reply keyboards.
type RichResponse struct {
	Text          string
	InlineButtons [][]InlineButton
	ReplyKeyboard [][]string
}

// InlineButton represents a clickable button in a message.
type InlineButton struct {
	Text         string
	CallbackData string
	URL          string
}

// VerifyRequest holds webhook verification parameters.
type VerifyRequest struct {
	Mode        string
	Token       string
	Challenge   string
	VerifyToken string
	BotToken    string // For Telegram URL-based verification
}

// MessagingChannel abstracts platform-specific messaging operations.
type MessagingChannel interface {
	// ParseWebhook parses raw webhook body into normalized messages.
	ParseWebhook(body []byte) ([]NormalizedMessage, error)

	// SendResponse sends a rich response to a recipient on a given channel.
	SendResponse(ctx context.Context, channel *domain.Channel, to string, response RichResponse) error

	// DownloadMedia downloads media by its platform-specific ID.
	DownloadMedia(ctx context.Context, channel *domain.Channel, mediaID string) ([]byte, error)

	// VerifyWebhook handles platform-specific webhook verification.
	VerifyWebhook(req VerifyRequest) (string, error)
}
```

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/ports/messaging.go
git commit -m "feat: add MessagingChannel interface and normalized message types"
```

---

### Task 4: Refactor Meta Adapter to Implement `MessagingChannel`

**Files:**
- Modify: `internal/adapters/meta/client.go`
- Modify: `internal/handlers/meta_types.go` → Move to `internal/adapters/meta/types.go`
- Delete: `internal/ports/meta.go` (replaced by `messaging.go`)

**Step 1: Move meta types to adapter package**

Create `internal/adapters/meta/types.go` with the contents of `internal/handlers/meta_types.go`, changing the package to `meta`:

```go
package meta

// MetaWebhookPayload represents the payload from WhatsApp Cloud API.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Field string `json:"field"`
	Value Value  `json:"value"`
}

type Value struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Contact struct {
	Profile Profile `json:"profile"`
	WaID    string  `json:"wa_id"`
}

type Profile struct {
	Name string `json:"name"`
}

type Message struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *Text  `json:"text,omitempty"`
	Audio     *Audio `json:"audio,omitempty"`
}

type Audio struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
}

type Text struct {
	Body string `json:"body"`
}
```

**Step 2: Rewrite meta/client.go to implement MessagingChannel**

```go
package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"blackonix/internal/domain"
	"blackonix/internal/ports"
)

const (
	graphAPIURL     = "https://graph.facebook.com/v21.0"
	metaHTTPTimeout = 30 * time.Second
	maxMediaSize    = 25 * 1024 * 1024 // 25 MB
)

type client struct {
	httpClient *http.Client
}

func NewMetaChannel() ports.MessagingChannel {
	return &client{
		httpClient: &http.Client{Timeout: metaHTTPTimeout},
	}
}

func (c *client) ParseWebhook(body []byte) ([]ports.NormalizedMessage, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal meta webhook: %w", err)
	}

	var messages []ports.NormalizedMessage
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			// Find contact name if available
			contactNames := make(map[string]string)
			for _, c := range change.Value.Contacts {
				contactNames[c.WaID] = c.Profile.Name
			}

			for _, msg := range change.Value.Messages {
				nm := ports.NormalizedMessage{
					ExternalID:  msg.ID,
					ChannelType: domain.ChannelWhatsApp,
					From:        msg.From,
					FromName:    contactNames[msg.From],
					Timestamp:   time.Now(),
				}

				switch msg.Type {
				case "text":
					nm.Type = ports.MsgText
					if msg.Text != nil {
						nm.Text = msg.Text.Body
					}
				case "audio":
					nm.Type = ports.MsgAudio
					if msg.Audio != nil {
						nm.MediaID = msg.Audio.ID
						nm.MediaMimeType = msg.Audio.MimeType
					}
				default:
					nm.Type = ports.MsgText
					nm.Text = "[mensagem não-textual recebida]"
				}

				messages = append(messages, nm)
			}
		}
	}

	return messages, nil
}

func (c *client) SendResponse(ctx context.Context, channel *domain.Channel, to string, response ports.RichResponse) error {
	token := channel.Credentials.Get("meta_token")
	phoneNumberID := channel.Credentials.Get("phone_number_id")

	url := fmt.Sprintf("%s/%s/messages", graphAPIURL, phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": response.Text,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("meta API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *client) DownloadMedia(ctx context.Context, channel *domain.Channel, mediaID string) ([]byte, error) {
	token := channel.Credentials.Get("meta_token")

	// 1. Get media URL
	url := fmt.Sprintf("%s/%s", graphAPIURL, mediaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create media info request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get media info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("meta media info API returned status %d", resp.StatusCode)
	}

	var mediaInfo struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mediaInfo); err != nil {
		return nil, fmt.Errorf("decode media info: %w", err)
	}

	// 2. Download the media file
	mediaReq, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaInfo.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create media download request: %w", err)
	}
	mediaReq.Header.Set("Authorization", "Bearer "+token)

	mediaResp, err := c.httpClient.Do(mediaReq)
	if err != nil {
		return nil, fmt.Errorf("download media: %w", err)
	}
	defer mediaResp.Body.Close()

	if mediaResp.StatusCode >= 400 {
		return nil, fmt.Errorf("meta media download returned status %d", mediaResp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(mediaResp.Body, maxMediaSize))
	if err != nil {
		return nil, fmt.Errorf("read media body: %w", err)
	}

	return data, nil
}

func (c *client) VerifyWebhook(req ports.VerifyRequest) (string, error) {
	if req.Mode == "subscribe" && req.Token == req.VerifyToken {
		return req.Challenge, nil
	}
	return "", fmt.Errorf("webhook verification failed")
}
```

**Step 3: Delete old files**

Delete `internal/ports/meta.go` and `internal/handlers/meta_types.go`.

**Step 4: Build — expect errors**

Run: `go build ./...`
Expected: Compilation errors in `webhook.go`, `audio_transcriber.go`, and `main.go` since they reference the old `ports.MetaAPI`. We'll fix these in the next tasks.

**Step 5: Commit**

```bash
git add internal/adapters/meta/ internal/ports/
git rm internal/ports/meta.go internal/handlers/meta_types.go
git commit -m "refactor: Meta adapter implements MessagingChannel interface"
```

---

### Task 5: Refactor AudioTranscriberTool for Channel Abstraction

**Files:**
- Modify: `internal/plugins/audio_transcriber.go`

The AudioTranscriberTool currently depends on `ports.MetaAPI` directly. Refactor it to accept a `ports.MessagingChannel` and a `*domain.Channel` so it works with any platform's media download.

**Step 1: Update AudioTranscriberTool**

```go
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"blackonix/internal/domain"
	"blackonix/internal/ports"
)

const (
	whisperURL         = "https://api.openai.com/v1/audio/transcriptions"
	whisperHTTPTimeout = 45 * time.Second
)

// AudioTranscriberTool transcribes audio messages using OpenAI Whisper API.
type AudioTranscriberTool struct {
	apiKey     string
	httpClient *http.Client
}

func NewAudioTranscriberTool(apiKey string) *AudioTranscriberTool {
	return &AudioTranscriberTool{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: whisperHTTPTimeout},
	}
}

// TranscribeFromChannel downloads audio from a channel and transcribes it.
func (t *AudioTranscriberTool) TranscribeFromChannel(ctx context.Context, ch ports.MessagingChannel, channel *domain.Channel, mediaID string) (string, error) {
	audioData, err := ch.DownloadMedia(ctx, channel, mediaID)
	if err != nil {
		return "", fmt.Errorf("download audio: %w", err)
	}

	transcript, err := t.transcribe(ctx, audioData)
	if err != nil {
		return "", fmt.Errorf("transcribe audio: %w", err)
	}

	return transcript, nil
}

func (t *AudioTranscriberTool) Name() string {
	return "transcribe_audio"
}

func (t *AudioTranscriberTool) Description() string {
	return "Transcreve uma mensagem de áudio recebida para texto usando IA (Whisper)."
}

func (t *AudioTranscriberTool) ParametersSchema() interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *AudioTranscriberTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	return "", fmt.Errorf("use TranscribeFromChannel instead")
}

func (t *AudioTranscriberTool) transcribe(ctx context.Context, audioData []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "audio.ogg")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(audioData)); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	if err := writer.WriteField("model", "whisper-1"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}

	if err := writer.WriteField("language", "pt"); err != nil {
		return "", fmt.Errorf("write language field: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create whisper request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode whisper response: %w", err)
	}

	return result.Text, nil
}
```

**Step 2: Commit**

```bash
git add internal/plugins/audio_transcriber.go
git commit -m "refactor: AudioTranscriberTool uses MessagingChannel for media download"
```

---

### Task 6: Refactor Webhook Handler to Be Channel-Agnostic

**Files:**
- Modify: `internal/handlers/webhook.go`

This is the largest refactor. The handler no longer knows about Meta-specific types. It uses `MessagingChannel` and `ChannelRepository`.

**Step 1: Rewrite webhook.go**

```go
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
	// Validate bot token from URL
	urlToken := c.Params("token")
	if urlToken == "" {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	ch := h.channels[domain.ChannelTelegram]
	if _, err := ch.VerifyWebhook(ports.VerifyRequest{BotToken: urlToken}); err != nil {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	return h.handleWebhook(c, domain.ChannelTelegram)
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
	// 1. Find Channel by sender info
	// For WhatsApp, we need to look up by external ID from the webhook payload.
	// For Telegram, the bot token was already validated.
	// We look up channel by external ID or by the from field depending on platform.
	channel, err := h.resolveChannel(ctx, msg)
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

// resolveChannel finds the Channel record based on the incoming message.
// For WhatsApp, the external ID comes from the webhook entry ID (WABA ID).
// For Telegram, we extract the bot ID from the message context.
func (h *WebhookHandler) resolveChannel(ctx context.Context, msg ports.NormalizedMessage) (*domain.Channel, error) {
	// The NormalizedMessage.ExternalID is the message ID, not the channel ID.
	// We need a way to carry the channel external ID through parsing.
	// This is handled by ParseWebhook setting it on the message — see channelExternalID field below.
	// For now, we use a simple approach: look up active channels and match.
	return h.channelRepo.FindByExternalID(ctx, msg.ExternalID)
}
```

**Important note:** The `resolveChannel` function needs the channel's external ID (WABA ID or Bot ID), not the message's external ID. We need to add a `ChannelExternalID` field to `NormalizedMessage`. Update `internal/ports/messaging.go`:

Add to `NormalizedMessage`:
```go
type NormalizedMessage struct {
	ExternalID        string             // Message ID on the source platform
	ChannelExternalID string             // Channel identifier (WABA ID or Bot ID)
	ChannelType       domain.ChannelType
	// ... rest unchanged
}
```

And update `resolveChannel`:
```go
func (h *WebhookHandler) resolveChannel(ctx context.Context, msg ports.NormalizedMessage) (*domain.Channel, error) {
	return h.channelRepo.FindByExternalID(ctx, msg.ChannelExternalID)
}
```

And update Meta's `ParseWebhook` to set `ChannelExternalID: entry.ID` (the WABA ID).

**Step 2: Build — may still have errors from main.go**

Run: `go build ./...`
Expected: Errors in `cmd/server/main.go` (old constructor). Fixed in next task.

**Step 3: Commit**

```bash
git add internal/handlers/webhook.go internal/ports/messaging.go
git commit -m "refactor: webhook handler uses MessagingChannel abstraction"
```

---

### Task 7: Create Telegram Adapter

**Files:**
- Create: `internal/adapters/telegram/types.go`
- Create: `internal/adapters/telegram/client.go`

**Step 1: Create Telegram types**

```go
package telegram

// Update represents a Telegram Bot API Update object.
type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *TGMessage    `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type TGMessage struct {
	MessageID int      `json:"message_id"`
	From      *TGUser  `json:"from,omitempty"`
	Chat      TGChat   `json:"chat"`
	Date      int64    `json:"date"`
	Text      string   `json:"text,omitempty"`
	Voice     *TGVoice `json:"voice,omitempty"`
	Audio     *TGAudio `json:"audio,omitempty"`
	Photo     []TGPhotoSize `json:"photo,omitempty"`
	Video     *TGVideo `json:"video,omitempty"`
}

type TGUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type TGChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type TGVoice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
}

type TGAudio struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
}

type TGPhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int    `json:"file_size,omitempty"`
}

type TGVideo struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
}

type CallbackQuery struct {
	ID      string     `json:"id"`
	From    TGUser     `json:"from"`
	Message *TGMessage `json:"message,omitempty"`
	Data    string     `json:"data,omitempty"`
}

// API response types

type TGFileResponse struct {
	OK     bool   `json:"ok"`
	Result TGFile `json:"result"`
}

type TGFile struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int    `json:"file_size,omitempty"`
}

// Keyboard types for rich responses

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type ReplyKeyboardMarkup struct {
	Keyboard        [][]KeyboardButton `json:"keyboard"`
	ResizeKeyboard  bool               `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard bool               `json:"one_time_keyboard,omitempty"`
}

type KeyboardButton struct {
	Text string `json:"text"`
}
```

**Step 2: Create Telegram client**

```go
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"blackonix/internal/domain"
	"blackonix/internal/ports"
)

const (
	telegramAPIBase    = "https://api.telegram.org"
	telegramHTTPTimeout = 30 * time.Second
	maxMediaSize       = 20 * 1024 * 1024 // 20 MB (Telegram bot limit)
)

type client struct {
	httpClient *http.Client
}

func NewTelegramChannel() ports.MessagingChannel {
	return &client{
		httpClient: &http.Client{Timeout: telegramHTTPTimeout},
	}
}

func (c *client) ParseWebhook(body []byte) ([]ports.NormalizedMessage, error) {
	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, fmt.Errorf("unmarshal telegram update: %w", err)
	}

	var messages []ports.NormalizedMessage

	if update.CallbackQuery != nil {
		cq := update.CallbackQuery
		chatID := ""
		if cq.Message != nil {
			chatID = strconv.FormatInt(cq.Message.Chat.ID, 10)
		}
		messages = append(messages, ports.NormalizedMessage{
			ExternalID:        cq.ID,
			ChannelExternalID: "", // Resolved via bot token in URL
			ChannelType:       domain.ChannelTelegram,
			From:              chatID,
			FromName:          buildName(cq.From),
			Type:              ports.MsgText,
			Text:              cq.Data,
			Timestamp:         time.Now(),
		})
		return messages, nil
	}

	if update.Message != nil {
		msg := update.Message
		chatID := strconv.FormatInt(msg.Chat.ID, 10)
		fromName := ""
		if msg.From != nil {
			fromName = buildName(*msg.From)
		}

		nm := ports.NormalizedMessage{
			ExternalID:        strconv.Itoa(msg.MessageID),
			ChannelExternalID: "", // Resolved via bot token in URL
			ChannelType:       domain.ChannelTelegram,
			From:              chatID,
			FromName:          fromName,
			Timestamp:         time.Unix(msg.Date, 0),
		}

		switch {
		case msg.Voice != nil:
			nm.Type = ports.MsgAudio
			nm.MediaID = msg.Voice.FileID
			nm.MediaMimeType = msg.Voice.MimeType
		case msg.Audio != nil:
			nm.Type = ports.MsgAudio
			nm.MediaID = msg.Audio.FileID
			nm.MediaMimeType = msg.Audio.MimeType
		case len(msg.Photo) > 0:
			// Pick largest photo
			largest := msg.Photo[len(msg.Photo)-1]
			nm.Type = ports.MsgPhoto
			nm.MediaID = largest.FileID
		case msg.Video != nil:
			nm.Type = ports.MsgVideo
			nm.MediaID = msg.Video.FileID
			nm.MediaMimeType = msg.Video.MimeType
		default:
			nm.Type = ports.MsgText
			nm.Text = msg.Text
		}

		messages = append(messages, nm)
	}

	return messages, nil
}

func (c *client) SendResponse(ctx context.Context, channel *domain.Channel, to string, response ports.RichResponse) error {
	token := channel.Credentials.Get("bot_token")
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, token)

	payload := map[string]interface{}{
		"chat_id": to,
		"text":    response.Text,
	}

	// Add inline keyboard if present
	if len(response.InlineButtons) > 0 {
		var keyboard [][]InlineKeyboardButton
		for _, row := range response.InlineButtons {
			var kbRow []InlineKeyboardButton
			for _, btn := range row {
				kbBtn := InlineKeyboardButton{
					Text:         btn.Text,
					CallbackData: btn.CallbackData,
				}
				if btn.URL != "" {
					kbBtn.URL = btn.URL
					kbBtn.CallbackData = "" // URL buttons can't have callback_data
				}
				kbRow = append(kbRow, kbBtn)
			}
			keyboard = append(keyboard, kbRow)
		}
		payload["reply_markup"] = InlineKeyboardMarkup{InlineKeyboard: keyboard}
	} else if len(response.ReplyKeyboard) > 0 {
		// Add reply keyboard if present (and no inline buttons)
		var keyboard [][]KeyboardButton
		for _, row := range response.ReplyKeyboard {
			var kbRow []KeyboardButton
			for _, text := range row {
				kbRow = append(kbRow, KeyboardButton{Text: text})
			}
			keyboard = append(keyboard, kbRow)
		}
		payload["reply_markup"] = ReplyKeyboardMarkup{
			Keyboard:        keyboard,
			ResizeKeyboard:  true,
			OneTimeKeyboard: true,
		}
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *client) DownloadMedia(ctx context.Context, channel *domain.Channel, mediaID string) ([]byte, error) {
	token := channel.Credentials.Get("bot_token")

	// 1. Get file path
	fileURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", telegramAPIBase, token, mediaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create getFile request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getFile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("telegram getFile returned status %d", resp.StatusCode)
	}

	var fileResp TGFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return nil, fmt.Errorf("decode getFile response: %w", err)
	}

	if !fileResp.OK {
		return nil, fmt.Errorf("telegram getFile returned not ok")
	}

	// 2. Download the file
	downloadURL := fmt.Sprintf("%s/file/bot%s/%s", telegramAPIBase, token, fileResp.Result.FilePath)

	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	downloadResp, err := c.httpClient.Do(downloadReq)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode >= 400 {
		return nil, fmt.Errorf("telegram file download returned status %d", downloadResp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(downloadResp.Body, maxMediaSize))
	if err != nil {
		return nil, fmt.Errorf("read file body: %w", err)
	}

	return data, nil
}

func (c *client) VerifyWebhook(req ports.VerifyRequest) (string, error) {
	// Telegram verification: the bot token in the URL must match a known channel.
	// The actual validation happens in the handler by looking up the channel.
	if req.BotToken != "" {
		return "ok", nil
	}
	return "", fmt.Errorf("missing bot token")
}

func buildName(user TGUser) string {
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	return name
}
```

**Step 3: Build and verify**

Run: `go build ./...`
Expected: May still fail on `cmd/server/main.go`. Fixed in next task.

**Step 4: Commit**

```bash
git add internal/adapters/telegram/
git commit -m "feat: add Telegram adapter implementing MessagingChannel"
```

---

### Task 8: Update `main.go` and Routing

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `cmd/seed/main.go` (update seed to create Channel records)

**Step 1: Update main.go**

```go
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
	// 1. Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Database
	db, err := repository.NewDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 3. Repositories
	channelRepo := repository.NewChannelRepository(db)
	contactRepo := repository.NewContactRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// 4. Adapters (MessagingChannel implementations)
	channelAdapters := map[domain.ChannelType]ports.MessagingChannel{
		domain.ChannelWhatsApp: meta.NewMetaChannel(),
		domain.ChannelTelegram: telegram.NewTelegramChannel(),
	}

	rocketChatAPI := rocketchat.NewRocketChatClient()
	llmClient := llm.NewOpenAIClient(cfg.LLMApiKey, cfg.LLMModel)

	// 5. Core
	stateMachine := state.NewMachine(sessionRepo)

	// 6. Tool Registry (global plugins)
	registry := agent.NewToolRegistry()
	audioTranscriber := plugins.NewAudioTranscriberTool(cfg.LLMApiKey)

	// 7. Fiber App
	app := fiber.New(fiber.Config{
		AppName: "BlackOnix Agentic Middleware",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	// 8. Webhook Handler with DI
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

	// 9. Routes
	// WhatsApp
	app.Get("/webhook/whatsapp", webhookHandler.VerifyWhatsAppWebhook)
	app.Post("/webhook/whatsapp", webhookHandler.HandleWhatsAppWebhook)

	// Telegram
	app.Post("/webhook/telegram/:token", webhookHandler.HandleTelegramWebhook)

	// Legacy routes (backward compatibility — remove after migration)
	app.Get("/webhook", webhookHandler.VerifyWhatsAppWebhook)
	app.Post("/webhook", webhookHandler.HandleWhatsAppWebhook)

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
```

**Step 2: Update seed script**

Read `cmd/seed/main.go` first, then update it to create Channel records instead of setting WabaID/MetaToken on Tenant. The seed should:
1. Create Tenant (without WabaID/MetaToken)
2. Create a WhatsApp Channel linked to that Tenant with credentials JSON

**Step 3: Build and verify**

Run: `go build ./...`
Expected: Clean build. All compilation errors resolved.

**Step 4: Commit**

```bash
git add cmd/server/main.go cmd/seed/main.go
git commit -m "refactor: main.go uses channel abstraction with per-platform routes"
```

---

### Task 9: Handle Telegram Channel Resolution via Bot Token

**Files:**
- Modify: `internal/handlers/webhook.go`
- Modify: `internal/adapters/telegram/client.go`

The Telegram webhook URL contains the bot token (e.g., `/webhook/telegram/123456:ABC-DEF`). We need to pass this through so `resolveChannel` can look up the channel by the bot token's bot ID portion.

**Step 1: Extract bot ID from token**

The bot token format is `<bot_id>:<hash>`. The bot ID is the `ExternalID` stored in the Channel table.

Update `HandleTelegramWebhook` in `webhook.go`:

```go
func (h *WebhookHandler) HandleTelegramWebhook(c *fiber.Ctx) error {
	urlToken := c.Params("token")
	if urlToken == "" {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	// Validate this token corresponds to a known channel
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

func extractBotID(token string) string {
	for i, c := range token {
		if c == ':' {
			return token[:i]
		}
	}
	return token
}
```

Similarly, update `HandleWhatsAppWebhook` to extract WABA ID from the parsed messages (already set by Meta's ParseWebhook via `entry.ID`).

**Step 2: Build and verify**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/handlers/webhook.go
git commit -m "feat: Telegram webhook validates bot token and resolves channel"
```

---

### Task 10: Create Telegram Setup CLI

**Files:**
- Create: `cmd/telegram-setup/main.go`

This CLI registers the webhook URL with Telegram's `setWebhook` API.

**Step 1: Create the CLI**

```go
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
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL") // e.g., https://yourdomain.com/webhook/telegram/<bot_token>

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
```

**Step 2: Build and verify**

Run: `go build ./cmd/telegram-setup/`
Expected: Clean build.

**Step 3: Commit**

```bash
git add cmd/telegram-setup/
git commit -m "feat: add Telegram webhook setup CLI"
```

---

### Task 11: Migrate Tenant Data to Channel Model

**Files:**
- Modify: `internal/domain/tenant.go`
- Create: `cmd/migrate-channels/main.go` (one-time migration script)

**Step 1: Remove platform-specific fields from Tenant**

```go
package domain

import "time"

type Tenant struct {
	ID              string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name            string `gorm:"not null"`
	RocketChatURL   string `gorm:"column:rocketchat_url"`
	RocketChatToken string `gorm:"column:rocketchat_token"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
```

**Step 2: Create migration script**

Create `cmd/migrate-channels/main.go` that:
1. Reads all tenants with `waba_id` and `meta_token` (using raw SQL since fields will be removed from model)
2. Creates a Channel record for each tenant with the credentials
3. Drops the old columns

```go
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

	// Read existing tenants with old fields (raw SQL since model no longer has them)
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
			TenantID:    ot.ID,
			Type:        domain.ChannelWhatsApp,
			ExternalID:  ot.WabaID,
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

	// Drop old columns
	if err := db.Exec("ALTER TABLE tenants DROP COLUMN IF EXISTS waba_id, DROP COLUMN IF EXISTS meta_token").Error; err != nil {
		log.Printf("Failed to drop old columns (may already be removed): %v", err)
	}

	fmt.Println("Migration complete!")
}
```

**Step 3: Update TenantRepository — remove FindByWabaID**

In `internal/repository/interfaces.go`, remove `FindByWabaID` from `TenantRepository` interface.
In `internal/repository/tenant_repo.go`, remove the `FindByWabaID` method.

**Step 4: Build and verify**

Run: `go build ./...`
Expected: Clean build. All references to `FindByWabaID`, `MetaToken`, `WabaID` on Tenant are removed.

**Step 5: Commit**

```bash
git add internal/domain/tenant.go internal/repository/ cmd/migrate-channels/
git commit -m "refactor: remove platform fields from Tenant, add migration script"
```

---

### Task 12: Update Seed Script for Channel Model

**Files:**
- Modify: `cmd/seed/main.go`

**Step 1: Read current seed script**

Read `cmd/seed/main.go` to understand current structure.

**Step 2: Update seed to create Tenant + Channel**

The seed should:
1. Create a Tenant (name, rocketchat config only)
2. Create a WhatsApp Channel with credentials
3. Optionally create a Telegram Channel if env vars are set

**Step 3: Build and run seed**

Run: `go build ./cmd/seed/ && ./seed`
Expected: Creates tenant and channel records.

**Step 4: Commit**

```bash
git add cmd/seed/main.go
git commit -m "refactor: seed script creates Channel records"
```

---

### Task 13: Full Build and Integration Verification

**Step 1: Build all packages**

Run: `go build ./...`
Expected: Clean build with zero errors.

**Step 2: Verify the server starts**

Run: `go run cmd/server/main.go`
Expected: Server starts, logs show BlackOnix starting.

**Step 3: Test health endpoint**

Run: `curl http://localhost:3000/health`
Expected: `{"status":"ok","service":"blackonix"}`

**Step 4: Run migration (if database has existing data)**

Run: `go run cmd/migrate-channels/main.go`
Expected: Migrates existing tenant WABA/token data to Channel records.

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete Telegram adapter with channel abstraction layer"
```

---

## Summary of Changes

| Task | What | Files |
|------|------|-------|
| 1 | Channel domain model | `domain/channel.go`, `domain/contact.go`, `repository/database.go` |
| 2 | ChannelRepository | `repository/interfaces.go`, `repository/channel_repo.go` |
| 3 | MessagingChannel interface | `ports/messaging.go` |
| 4 | Meta adapter refactor | `adapters/meta/client.go`, `adapters/meta/types.go` |
| 5 | AudioTranscriber refactor | `plugins/audio_transcriber.go` |
| 6 | Webhook handler refactor | `handlers/webhook.go` |
| 7 | Telegram adapter | `adapters/telegram/client.go`, `adapters/telegram/types.go` |
| 8 | main.go + routing | `cmd/server/main.go` |
| 9 | Telegram token resolution | `handlers/webhook.go` |
| 10 | Telegram setup CLI | `cmd/telegram-setup/main.go` |
| 11 | Tenant migration | `domain/tenant.go`, `cmd/migrate-channels/main.go` |
| 12 | Seed script update | `cmd/seed/main.go` |
| 13 | Full build verification | All |
