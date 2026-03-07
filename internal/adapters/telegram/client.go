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
	telegramAPIBase     = "https://api.telegram.org"
	telegramHTTPTimeout = 30 * time.Second
	maxMediaSize        = 20 * 1024 * 1024 // 20 MB (Telegram bot limit)
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
			ExternalID:  cq.ID,
			ChannelType: domain.ChannelTelegram,
			From:        chatID,
			FromName:    buildName(cq.From),
			Type:        ports.MsgText,
			Text:        cq.Data,
			Timestamp:   time.Now(),
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
			ExternalID:  strconv.Itoa(msg.MessageID),
			ChannelType: domain.ChannelTelegram,
			From:        chatID,
			FromName:    fromName,
			Timestamp:   time.Unix(msg.Date, 0),
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
					kbBtn.CallbackData = ""
				}
				kbRow = append(kbRow, kbBtn)
			}
			keyboard = append(keyboard, kbRow)
		}
		payload["reply_markup"] = InlineKeyboardMarkup{InlineKeyboard: keyboard}
	} else if len(response.ReplyKeyboard) > 0 {
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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(respBody))
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
