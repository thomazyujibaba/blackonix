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
	maxMediaSize    = 25 * 1024 * 1024
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

			contactNames := make(map[string]string)
			for _, ct := range change.Value.Contacts {
				contactNames[ct.WaID] = ct.Profile.Name
			}

			for _, msg := range change.Value.Messages {
				nm := ports.NormalizedMessage{
					ExternalID:        msg.ID,
					ChannelExternalID: entry.ID,
					ChannelType:       domain.ChannelWhatsApp,
					From:              msg.From,
					FromName:          contactNames[msg.From],
					Timestamp:         time.Now(),
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
