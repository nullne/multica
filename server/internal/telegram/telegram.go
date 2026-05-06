package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// Bot wraps the Telegram Bot API.
type Bot struct {
	token   string
	baseURL string // defaults to "https://api.telegram.org"
}

// NewBotFromEnv reads TELEGRAM_BOT_TOKEN from the environment.
// Returns nil when the token is not configured.
func NewBotFromEnv() *Bot {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return nil
	}
	return &Bot{token: token}
}

// NewBotWithURL creates a Bot with a custom base URL, useful for testing.
func NewBotWithURL(token, baseURL string) *Bot {
	return &Bot{token: token, baseURL: strings.TrimRight(baseURL, "/")}
}

// SendMessage delivers a plain-text message to the given chat_id.
// chat_id may be a numeric ID string or a @username.
func (b *Bot) SendMessage(chatID, text string) error {
	if b == nil {
		return nil
	}
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	base := b.baseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", base, b.token)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned %d", resp.StatusCode)
	}
	return nil
}

// SendMessageAsync delivers a message in the background, logging any errors.
func (b *Bot) SendMessageAsync(chatID, text string) {
	if b == nil {
		return
	}
	go func() {
		if err := b.SendMessage(chatID, text); err != nil {
			slog.Warn("telegram delivery failed", "chat_id", chatID, "error", err)
		}
	}()
}
