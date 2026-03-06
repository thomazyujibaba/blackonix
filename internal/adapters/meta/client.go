package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"blackonix/internal/ports"
)

const graphAPIURL = "https://graph.facebook.com/v21.0"

type client struct {
	httpClient *http.Client
}

func NewMetaClient() ports.MetaAPI {
	return &client{
		httpClient: &http.Client{},
	}
}

func (c *client) SendTextMessage(ctx context.Context, token, phoneNumberID, to, body string) error {
	url := fmt.Sprintf("%s/%s/messages", graphAPIURL, phoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": body,
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

func (c *client) DownloadMedia(ctx context.Context, token, mediaID string) ([]byte, error) {
	// 1. Obter URL do media
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

	// 2. Baixar o arquivo de mídia
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

	data, err := io.ReadAll(mediaResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read media body: %w", err)
	}

	return data, nil
}

func (c *client) VerifyWebhook(mode, token, challenge, verifyToken string) (string, error) {
	if mode == "subscribe" && token == verifyToken {
		return challenge, nil
	}
	return "", fmt.Errorf("webhook verification failed")
}
