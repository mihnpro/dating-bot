package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dating-bot/chat-service/internal/service"
	gorillaws "github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Client is a single WebSocket connection.
type Client struct {
	hub         *Hub
	conn        *gorillaws.Conn
	userID      int64
	send        chan []byte
	chatService *service.ChatService
}

// wsClientMessage is sent by the browser to the server.
type wsClientMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
}

// ServeWS upgrades the HTTP connection and starts the read/write pumps.
func ServeWS(hub *Hub, chatSvc *service.ChatService, w http.ResponseWriter, r *http.Request, userID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Warn("websocket upgrade failed")
		return
	}
	client := &Client{
		hub:         hub,
		conn:        conn,
		userID:      userID,
		send:        make(chan []byte, 256),
		chatService: chatSvc,
	}
	hub.register <- client
	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg wsClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		c.handleMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gorillaws.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(gorillaws.TextMessage, payload); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gorillaws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg wsClientMessage) {
	ctx := context.Background()
	switch msg.Type {
	case "send_message":
		if msg.ConversationID == "" || msg.Content == "" {
			c.sendError("conversation_id and content are required")
			return
		}
		if _, err := c.chatService.SendMessage(ctx, msg.ConversationID, c.userID, msg.Content); err != nil {
			logrus.WithError(err).Warn("send message failed")
			c.sendError(err.Error())
		}

	case "mark_read":
		if msg.ConversationID == "" {
			return
		}
		_ = c.chatService.MarkAsRead(ctx, msg.ConversationID, c.userID)

	case "ping":
		payload, _ := json.Marshal(map[string]string{"type": "pong"})
		c.send <- payload
	}
}

func (c *Client) sendError(msg string) {
	payload, _ := json.Marshal(map[string]string{"type": "error", "error": msg})
	select {
	case c.send <- payload:
	default:
	}
}
