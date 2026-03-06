package handlers

// MetaWebhookPayload representa o payload recebido da WhatsApp Cloud API.
type MetaWebhookPayload struct {
	Object string      `json:"object"`
	Entry  []MetaEntry `json:"entry"`
}

type MetaEntry struct {
	ID      string       `json:"id"`
	Changes []MetaChange `json:"changes"`
}

type MetaChange struct {
	Field string    `json:"field"`
	Value MetaValue `json:"value"`
}

type MetaValue struct {
	MessagingProduct string        `json:"messaging_product"`
	Metadata         MetaMetadata  `json:"metadata"`
	Contacts         []MetaContact `json:"contacts"`
	Messages         []MetaMessage `json:"messages"`
}

type MetaMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type MetaContact struct {
	Profile MetaProfile `json:"profile"`
	WaID    string      `json:"wa_id"`
}

type MetaProfile struct {
	Name string `json:"name"`
}

type MetaMessage struct {
	From      string     `json:"from"`
	ID        string     `json:"id"`
	Timestamp string     `json:"timestamp"`
	Type      string     `json:"type"`
	Text      *MetaText  `json:"text,omitempty"`
	Audio     *MetaAudio `json:"audio,omitempty"`
}

type MetaAudio struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
}

type MetaText struct {
	Body string `json:"body"`
}