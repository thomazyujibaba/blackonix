# Telegram Adapter Design — BlackOnix

**Date:** 2026-03-06
**Status:** Approved

## Summary

Add Telegram as a second messaging channel to BlackOnix, introducing a channel abstraction layer that decouples the core processing pipeline from any specific messaging platform.

## Requirements

- Webhook-only (no long polling)
- Message types: text, audio, photo, video
- Rich responses: inline buttons, custom reply keyboards
- Multi-tenant: one tenant can have multiple channels (WhatsApp + Telegram)
- No external Telegram libraries — direct HTTP calls to Bot API

## Architecture: Channel Abstraction (Approach A)

### 1. Domain — `Channel` Model

New `Channel` entity replaces per-platform fields on `Tenant`:

```go
type ChannelType string
const (
    ChannelWhatsApp ChannelType = "whatsapp"
    ChannelTelegram ChannelType = "telegram"
)

type Channel struct {
    ID          string      // UUID PK
    TenantID    string      // FK to Tenant
    Type        ChannelType
    Credentials JSON        // Platform-specific credentials
    ExternalID  string      // Unique: WABA ID or Telegram Bot ID
    Active      bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Credentials format:**
- WhatsApp: `{"meta_token":"...", "waba_id":"...", "phone_number_id":"..."}`
- Telegram: `{"bot_token":"..."}`

**Tenant impact:** Remove `WabaID` and `MetaToken` fields (migrate to Channel). Keep `RocketChatURL` and `RocketChatToken` on Tenant (human handoff config, not channel-specific).

**Contact impact:** Add `ChannelID` field to associate contacts with their originating channel.

### 2. Ports — `MessagingChannel` Interface

Normalized message struct (channel-agnostic):

```go
type MessageType string // text, audio, photo, video

type NormalizedMessage struct {
    ExternalID    string
    From          string      // Phone number or chat_id
    FromName      string
    Type          MessageType
    Text          string
    MediaID       string
    MediaMimeType string
    Timestamp     time.Time
}

type RichResponse struct {
    Text          string
    InlineButtons [][]InlineButton
    ReplyKeyboard [][]string
}

type InlineButton struct {
    Text         string
    CallbackData string
    URL          string
}
```

Channel interface:

```go
type MessagingChannel interface {
    ParseWebhook(body []byte) ([]NormalizedMessage, error)
    SendResponse(ctx context.Context, channel *domain.Channel, to string, response RichResponse) error
    DownloadMedia(ctx context.Context, channel *domain.Channel, mediaID string) ([]byte, error)
    VerifyWebhook(req VerifyRequest) (string, error)
}
```

### 3. Handlers — Generic Webhook Routing

Routes:

```
GET  /webhook/whatsapp           -> Meta challenge verification
POST /webhook/whatsapp           -> WhatsApp updates
POST /webhook/telegram/:token    -> Telegram updates (token in URL for validation)
```

Generic processing flow:
1. Route identifies channel type
2. Lookup `Channel` by `ExternalID` (WABA ID or bot token)
3. `ParseWebhook(body)` -> `[]NormalizedMessage`
4. For each message: findOrCreate contact, findOrCreate session, transcribe audio if needed, process with orchestrator
5. Send response via `SendResponse()`

**Telegram callback_query:** Treated as a text message with `callback_data` as the body, so the orchestrator processes it naturally.

### 4. Telegram Adapter

File: `internal/adapters/telegram/client.go`

Direct HTTP calls to `https://api.telegram.org/bot<token>/`:

- **ParseWebhook:** Deserialize Telegram Update, extract `message` or `callback_query`, normalize
- **SendResponse:** Call `sendMessage` with `reply_markup` (InlineKeyboardMarkup or ReplyKeyboardMarkup)
- **DownloadMedia:** Call `getFile` to get file path, download from `https://api.telegram.org/file/bot<token>/<path>`
- **VerifyWebhook:** Validate token in URL matches channel's bot token

Message type mapping:
- `message.text` -> MsgText
- `message.voice` / `message.audio` -> MsgAudio
- `message.photo` -> MsgPhoto (largest resolution from photo array)
- `message.video` -> MsgVideo
- `callback_query` -> MsgText with callback_data

### 5. Security & Performance

- 30s timeout on Telegram API calls
- 20MB media download limit (Telegram bot file limit)
- Bot token in URL validated against channel credentials
- Reuse existing concurrency semaphore (20 goroutines max)

## File Map

| Component | File(s) |
|---|---|
| Domain `Channel` | `internal/domain/channel.go` |
| `NormalizedMessage`, `RichResponse`, `MessagingChannel` | `internal/ports/messaging.go` |
| Adapter Meta (refactor) | `internal/adapters/meta/client.go` |
| Adapter Telegram | `internal/adapters/telegram/client.go` |
| Telegram types | `internal/adapters/telegram/types.go` |
| Handler (refactor to generic) | `internal/handlers/webhook.go` |
| `ChannelRepository` | `internal/repository/channel_repo.go` |
| Config | `internal/config/config.go` |
| Telegram setup CLI | `cmd/telegram-setup/main.go` |
| Migration | Automatic via GORM AutoMigrate |
