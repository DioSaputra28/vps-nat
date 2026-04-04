package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type telegramAdminNotifier struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

func NewTelegramAdminNotifier(botToken string, chatID string) AdminNotifier {
	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" {
		return nil
	}

	return &telegramAdminNotifier{
		botToken:   strings.TrimSpace(botToken),
		chatID:     strings.TrimSpace(chatID),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *telegramAdminNotifier) NotifyProvisionFailure(ctx context.Context, message string) error {
	form := url.Values{}
	form.Set("chat_id", n.chatID)
	form.Set("text", message)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", url.PathEscape(n.botToken))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram admin notify failed with status %d", resp.StatusCode)
	}

	return nil
}
