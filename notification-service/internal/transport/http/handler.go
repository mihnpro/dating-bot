package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/dating-bot/notification-service/internal/service"
)

// Handler exposes the notification REST API.
type Handler struct {
	svc *service.NotificationService
}

func NewHandler(svc *service.NotificationService) *Handler {
	return &Handler{svc: svc}
}

// GET /api/v1/notifications/{user_id}
// Query params: limit (default 20), offset (default 0)
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := pathParamInt64(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)

	notifications, err := h.svc.GetNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		logrus.WithError(err).Error("get notifications")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	unread, _ := h.svc.CountUnread(r.Context(), userID)

	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": notifications,
		"unread_count":  unread,
	})
}

// POST /api/v1/notifications/{id}/read
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id, err := pathParamInt64(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	if err := h.svc.MarkRead(r.Context(), id); err != nil {
		logrus.WithError(err).Error("mark notification as read")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// pathParamInt64 extracts a named path parameter from the Go 1.22+ ServeMux pattern.
func pathParamInt64(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	return strconv.ParseInt(raw, 10, 64)
}

func queryInt(r *http.Request, name string, defaultVal int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
