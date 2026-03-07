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
	ExternalID        string             // Message ID on the source platform
	ChannelExternalID string             // Channel identifier (WABA ID or Bot ID)
	ChannelType       domain.ChannelType
	From              string      // Phone number (WhatsApp) or chat ID (Telegram)
	FromName          string
	Type              MessageType
	Text              string
	MediaID           string
	MediaMimeType     string
	Timestamp         time.Time
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
