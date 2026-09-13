package chat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramRoundTripFunc func(*http.Request) (*http.Response, error)

func (f telegramRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTelegramNetworkErrorsRedactToken(t *testing.T) {
	const token = "123456:synthetic-test-token"
	for _, method := range []string{"getMe", "sendMessage", "getUpdates"} {
		t.Run(method, func(t *testing.T) {
			client := &http.Client{Transport: telegramRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				// Redaction must not alter the actual authenticated request.
				if !strings.Contains(req.URL.Path, token) {
					t.Fatal("request lost its authentication token")
				}
				if strings.HasSuffix(req.URL.Path, "/"+method) {
					return nil, &net.DNSError{Err: "no such host", Name: "api.telegram.org", IsNotFound: true}
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"id":123456,"is_bot":true,"first_name":"Test"}}`))}, nil
			})}
			bot, err := newTelegramBot(token, client)
			if method != "getMe" {
				if err != nil {
					t.Fatal(err)
				}
				if method == "sendMessage" {
					_, err = bot.Send(tgbotapi.NewMessage(1, "hello"))
				} else {
					_, err = bot.GetUpdates(tgbotapi.NewUpdate(0))
				}
			}
			if err == nil {
				t.Fatal("expected DNS error")
			}
			var log bytes.Buffer
			slog.New(slog.NewTextHandler(&log, nil)).Error("telegram request failed", "error", err)
			for _, output := range []string{fmt.Sprintf("%v", err), log.String()} {
				if strings.Contains(output, token) || !strings.Contains(output, "[REDACTED]") {
					t.Fatalf("unsafe error output: %s", output)
				}
				if !strings.Contains(output, "no such host") || !strings.Contains(output, "api.telegram.org") || !strings.Contains(output, method) {
					t.Fatalf("lost diagnostic context: %s", output)
				}
			}
			for cause := err; cause != nil; cause = errors.Unwrap(cause) {
				if strings.Contains(cause.Error(), token) {
					t.Fatal("credential remains in the error chain")
				}
			}
		})
	}
}
