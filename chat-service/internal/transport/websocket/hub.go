package websocket

import (
	"sync"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages.
type Hub struct {
	// user_id → set of clients (a user may have multiple tabs open)
	clients map[int64]map[*Client]struct{}
	mu      sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastPayload
}

type broadcastPayload struct {
	recipientIDs []int64
	data         []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int64]map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan broadcastPayload, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[c.userID]; !ok {
				h.clients[c.userID] = make(map[*Client]struct{})
			}
			h.clients[c.userID][c] = struct{}{}
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.clients[c.userID]; ok {
				delete(conns, c)
				if len(conns) == 0 {
					delete(h.clients, c.userID)
				}
			}
			h.mu.Unlock()
			close(c.send)

		case bp := <-h.broadcast:
			h.mu.RLock()
			for _, uid := range bp.recipientIDs {
				for c := range h.clients[uid] {
					select {
					case c.send <- bp.data:
					default:
						// slow client: drop the message to avoid blocking
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast implements service.Hub — delivers payload to all clients of given users.
func (h *Hub) Broadcast(recipientIDs []int64, payload []byte) {
	h.broadcast <- broadcastPayload{recipientIDs: recipientIDs, data: payload}
}
