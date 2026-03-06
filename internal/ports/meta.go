package ports

import "context"

// MetaAPI define a interface para comunicação com a WhatsApp Cloud API (Meta).
type MetaAPI interface {
	SendTextMessage(ctx context.Context, token, phoneNumberID, to, body string) error
	DownloadMedia(ctx context.Context, token, mediaID string) ([]byte, error)
	VerifyWebhook(mode, token, challenge, verifyToken string) (string, error)
}
