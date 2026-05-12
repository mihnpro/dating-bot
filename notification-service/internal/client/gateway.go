package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GatewayClient calls the internal HTTP endpoint on gateway-service that
// delivers Telegram messages via the bot.
type GatewayClient struct {
	base       string
	httpClient *http.Client
}

func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		base: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Text       string `json:"text"`
}

// SendMessage asks the gateway-service to deliver a Telegram message to the user.
func (c *GatewayClient) SendMessage(ctx context.Context, telegramID int64, text string) error {
	body, err := json.Marshal(sendRequest{TelegramID: telegramID, Text: text})
	if err != nil {
		return fmt.Errorf("marshal send request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.base+"/internal/notify",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gateway POST /internal/notify: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned %d", resp.StatusCode)
	}
	return nil
}
