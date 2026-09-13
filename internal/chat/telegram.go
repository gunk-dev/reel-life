package chat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/patflynn/reel-life/internal/agent"
)

// telegramMaxMessageLength is the maximum length of a Telegram message.
const telegramMaxMessageLength = 4096

// BotAPI is the subset of tgbotapi.BotAPI methods used by the Telegram adapter.
// This allows constructor injection for testing without mocking the full API.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// Telegram sends and receives messages via the Telegram Bot API.
type Telegram struct {
	bot          BotAPI
	chatID       int64
	adminChatID  int64
	allowedUsers []int64
	logger       *slog.Logger
	history      *agent.HistoryStore

	mu      sync.Mutex
	threads map[string]int // threadKey → message ID
}

// NewTelegram creates a Telegram adapter and verifies the bot token is valid.
func NewTelegram(token string, chatID int64, adminChatID int64, allowedUsers []int64, logger *slog.Logger, history *agent.HistoryStore) (*Telegram, error) {
	bot, err := newTelegramBot(token, &http.Client{})
	if err != nil {
		return nil, fmt.Errorf("initialize telegram client: %w", err)
	}
	logger.Info("telegram bot authorized", "username", bot.Self.UserName)
	return &Telegram{
		bot:          bot,
		chatID:       chatID,
		adminChatID:  adminChatID,
		allowedUsers: allowedUsers,
		logger:       logger,
		history:      history,
		threads:      make(map[string]int),
	}, nil
}

// NewTelegramWithBot creates a Telegram adapter with a pre-configured BotAPI (for testing).
func NewTelegramWithBot(bot BotAPI, chatID int64, adminChatID int64, allowedUsers []int64, logger *slog.Logger, history *agent.HistoryStore) *Telegram {
	return &Telegram{
		bot:          bot,
		chatID:       chatID,
		adminChatID:  adminChatID,
		allowedUsers: allowedUsers,
		logger:       logger,
		history:      history,
		threads:      make(map[string]int),
	}
}

func (t *Telegram) Send(ctx context.Context, message string) error {
	chatID := atomic.LoadInt64(&t.chatID)
	if chatID == 0 {
		return fmt.Errorf("telegram chat ID not configured (waiting for first message)")
	}
	for _, chunk := range splitMessage(message, telegramMaxMessageLength) {
		msg := tgbotapi.NewMessage(chatID, chunk)
		if _, err := t.bot.Send(msg); err != nil {
			return fmt.Errorf("send telegram message: %w", err)
		}
	}
	t.logger.Debug("message sent to telegram")
	return nil
}

func (t *Telegram) SendThread(ctx context.Context, message string, threadKey string) error {
	chatID := atomic.LoadInt64(&t.chatID)
	if chatID == 0 {
		return fmt.Errorf("telegram chat ID not configured (waiting for first message)")
	}

	t.mu.Lock()
	replyTo := t.threads[threadKey]
	t.mu.Unlock()

	chunks := splitMessage(message, telegramMaxMessageLength)
	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(chatID, chunk)
		if replyTo != 0 {
			msg.ReplyToMessageID = replyTo
		}
		sent, err := t.bot.Send(msg)
		if err != nil {
			return fmt.Errorf("send telegram thread message: %w", err)
		}
		// Track the first message in a new thread so subsequent calls can reply to it.
		if replyTo == 0 {
			replyTo = sent.MessageID
			t.mu.Lock()
			t.threads[threadKey] = replyTo
			t.mu.Unlock()
		}
	}
	t.logger.Debug("thread message sent to telegram", "thread_key", threadKey)
	return nil
}

// SendAdmin sends a message to the admin chat ID if configured, otherwise
// falls back to SendThread on the main chat ID. Messages with the same
// threadKey are threaded together via reply chains.
func (t *Telegram) SendAdmin(ctx context.Context, message string, threadKey string) error {
	if t.adminChatID == 0 {
		return t.SendThread(ctx, message, threadKey)
	}

	t.mu.Lock()
	replyTo := t.threads[threadKey]
	t.mu.Unlock()

	chunks := splitMessage(message, telegramMaxMessageLength)
	for _, chunk := range chunks {
		msg := tgbotapi.NewMessage(t.adminChatID, chunk)
		if replyTo != 0 {
			msg.ReplyToMessageID = replyTo
		}
		sent, err := t.bot.Send(msg)
		if err != nil {
			return fmt.Errorf("send telegram admin message: %w", err)
		}
		// Track the first message in a new thread so subsequent calls can reply to it.
		if replyTo == 0 {
			replyTo = sent.MessageID
			t.mu.Lock()
			t.threads[threadKey] = replyTo
			t.mu.Unlock()
		}
	}
	t.logger.Debug("admin message sent to telegram", "admin_chat_id", t.adminChatID, "thread_key", threadKey)
	return nil
}

// Listen polls for incoming Telegram messages and dispatches them to the processor.
// It blocks until ctx is cancelled.
func (t *Telegram) Listen(ctx context.Context, processor MessageProcessor) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := t.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			t.bot.StopReceivingUpdates()
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message == nil {
				continue
			}
			t.handleUpdate(ctx, update, processor)
		}
	}
}

func (t *Telegram) handleUpdate(ctx context.Context, update tgbotapi.Update, processor MessageProcessor) {
	msg := update.Message

	// Access control: check user allowlist.
	if !t.isAllowed(msg.From.ID) {
		t.logger.Warn("ignoring message from unauthorized user",
			"user_id", msg.From.ID,
			"username", msg.From.UserName,
		)
		return
	}

	// Auto-capture chat ID from first incoming message.
	if atomic.CompareAndSwapInt64(&t.chatID, 0, msg.Chat.ID) {
		t.logger.Info("auto-captured telegram chat ID", "chat_id", msg.Chat.ID)
	}

	text := msg.Text

	// Strip bot command prefix.
	if msg.IsCommand() {
		text = strings.TrimSpace(msg.CommandArguments())
		if text == "" {
			// Bare command like /start with no arguments.
			reply := tgbotapi.NewMessage(msg.Chat.ID, "I'm your media curation assistant. Send me a message like \"search for Breaking Bad\" or \"what's downloading?\" to get started.")
			reply.ReplyToMessageID = msg.MessageID
			if _, err := t.bot.Send(reply); err != nil {
				t.logger.Error("failed to send help message", "error", err)
			}
			return
		}
	}

	if strings.TrimSpace(text) == "" {
		return
	}

	t.logger.Info("processing telegram message",
		"user", msg.From.UserName,
		"text_length", len(text),
	)

	// Send typing indicator.
	typing := tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping)
	if _, err := t.bot.Request(typing); err != nil {
		t.logger.Warn("failed to send typing indicator", "error", err)
	}

	chatKey := fmt.Sprintf("%d", msg.Chat.ID)
	var history []agent.Turn
	if t.history != nil {
		history = t.history.Get(chatKey).Turns()
	}

	response, err := processor.Process(ctx, text, history)
	if err != nil {
		t.logger.Error("agent processing failed", "error", err)
		response = "Sorry, I encountered an error processing your request. Please try again."
	}

	if t.history != nil {
		buf := t.history.Get(chatKey)
		buf.Add("user", text)
		buf.Add("assistant", response)
	}

	for _, chunk := range splitMessage(response, telegramMaxMessageLength) {
		reply := tgbotapi.NewMessage(msg.Chat.ID, chunk)
		reply.ReplyToMessageID = msg.MessageID
		if _, err := t.bot.Send(reply); err != nil {
			t.logger.Error("failed to send reply", "error", err)
		}
	}
}

// isAllowed checks whether the given user ID is in the allowlist.
// An empty allowlist rejects all users.
func isAllowed(userID int64, allowedUsers []int64) bool {
	for _, id := range allowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

func (t *Telegram) isAllowed(userID int64) bool {
	return isAllowed(userID, t.allowedUsers)
}

// splitMessage breaks text into chunks that fit within maxLen, preferring
// newline boundaries. If no newline is found within the limit, it splits
// at the hard limit.
func splitMessage(text string, maxLen int) []string {
	if text == "" {
		return nil
	}
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}

		// Try to find a newline boundary within the limit.
		cut := maxLen
		if idx := strings.LastIndex(text[:maxLen], "\n"); idx > 0 {
			cut = idx + 1 // include the newline in the current chunk
		}

		parts = append(parts, text[:cut])
		text = text[cut:]
	}
	return parts
}
