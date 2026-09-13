package chat

import (
	"errors"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Telegram embeds the token in the URL. Sanitize after http.Client.Do, which
// wraps transport failures with that URL, and before the SDK can log errors
// itself (notably in its background polling loop).
type telegramHTTPClient struct {
	client tgbotapi.HTTPClient
	token  string
}

func (c telegramHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err == nil {
		return resp, nil
	}
	message := err.Error()
	if c.token != "" {
		message = strings.ReplaceAll(message, c.token, "[REDACTED]")
	}
	// Do not wrap the original error: it still contains the credential and
	// could be exposed by unwrapping. The SDK only logs/returns these errors.
	return resp, errors.New(message)
}

func newTelegramBot(token string, client tgbotapi.HTTPClient) (*tgbotapi.BotAPI, error) {
	return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, telegramHTTPClient{
		client: client,
		token:  token,
	})
}
