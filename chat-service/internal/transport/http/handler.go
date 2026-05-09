package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dating-bot/chat-service/internal/domain/entity"
	"github.com/dating-bot/chat-service/internal/service"
	wshandler "github.com/dating-bot/chat-service/internal/transport/websocket"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	chatSvc  *service.ChatService
	tokenSvc *service.TokenService
	hub      *wshandler.Hub
}

func NewHandler(chatSvc *service.ChatService, tokenSvc *service.TokenService, hub *wshandler.Hub) *Handler {
	return &Handler{chatSvc: chatSvc, tokenSvc: tokenSvc, hub: hub}
}

// POST /api/v1/chat/conversations
func (h *Handler) CreateOrGetConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MatchID int64 `json:"match_id"`
		User1ID int64 `json:"user1_id"`
		User2ID int64 `json:"user2_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conv, err := h.chatSvc.GetOrCreateConversation(r.Context(), req.MatchID, req.User1ID, req.User2ID)
	if err != nil {
		logrus.WithError(err).Error("get or create conversation")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

// GET /api/v1/chat/conversations/{id}
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	conv, err := h.chatSvc.GetConversation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

// GET /api/v1/chat/conversations?user_id=X
// Returns conversations enriched with the other participant's name via gRPC.
func (h *Handler) ListUserConversations(w http.ResponseWriter, r *http.Request) {
	userID, err := parseQueryInt64(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	convs, err := h.chatSvc.GetUserConversationsEnriched(r.Context(), userID)
	if err != nil {
		logrus.WithError(err).Error("list user conversations")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if convs == nil {
		convs = []*service.EnrichedConversation{}
	}
	writeJSON(w, http.StatusOK, convs)
}

// GET /api/v1/chat/conversations/{id}/messages?limit=50&offset=0
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	convID := mux.Vars(r)["id"]
	limit, _ := parseQueryInt(r, "limit", 50)
	offset, _ := parseQueryInt(r, "offset", 0)

	msgs, err := h.chatSvc.GetMessages(r.Context(), convID, limit, offset)
	if err != nil {
		logrus.WithError(err).Error("get messages")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msgs == nil {
		msgs = []*entity.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// GET /api/v1/chat/token?user_id=X
func (h *Handler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	userID, err := parseQueryInt64(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	token := h.tokenSvc.Generate(userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":   token,
		"user_id": userID,
	})
}

// GET /ws?token=<token>
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	userID, err := h.tokenSvc.Validate(token)
	if err != nil {
		http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}
	wshandler.ServeWS(h.hub, h.chatSvc, w, r, userID)
}

// GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseQueryInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
}

func parseQueryInt(r *http.Request, key string, defaultVal int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal, err
	}
	return n, nil
}
