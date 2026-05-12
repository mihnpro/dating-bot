package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ConversationInfo holds the two participants of a chat conversation.
type ConversationInfo struct {
	ID      string
	User1ID int64
	User2ID int64
}

// ChatClient is an HTTP client for chat-service REST API.
// Used to resolve conversation_id → participant IDs when a message.sent event arrives.
type ChatClient struct {
	base       string
	httpClient *http.Client
}

func NewChatClient(baseURL string) *ChatClient {
	return &ChatClient{
		base: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetConversation fetches conversation details by ID from chat-service.
func (c *ChatClient) GetConversation(ctx context.Context, conversationID string) (*ConversationInfo, error) {
	url := fmt.Sprintf("%s/api/v1/chat/conversations/%s", c.base, conversationID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat-service GET conversation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("conversation %s not found", conversationID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat-service returned %d for conversation %s", resp.StatusCode, conversationID)
	}

	var body struct {
		Conversation struct {
			ID      string `json:"id"`
			User1ID int64  `json:"user1_id"`
			User2ID int64  `json:"user2_id"`
		} `json:"conversation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode conversation response: %w", err)
	}

	return &ConversationInfo{
		ID:      body.Conversation.ID,
		User1ID: body.Conversation.User1ID,
		User2ID: body.Conversation.User2ID,
	}, nil
}
