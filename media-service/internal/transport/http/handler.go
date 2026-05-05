package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/dating-bot/media-service/internal/service"
	"github.com/dating-bot/media-service/internal/transport/metrics"
)

const maxUploadSize = 10 << 20 // 10 MB

type Handler struct {
	svc *service.MediaService
	m   *metrics.Metrics
	log *logrus.Logger
	mux *http.ServeMux
}

func NewHandler(svc *service.MediaService, m *metrics.Metrics, log *logrus.Logger) http.Handler {
	h := &Handler{svc: svc, m: m, log: log, mux: http.NewServeMux()}
	h.registerRoutes()
	return h.withMiddleware(h.mux)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("POST /api/v1/media/upload", h.upload)
	h.mux.HandleFunc("GET /api/v1/media/user/{user_id}", h.listByUser)
	h.mux.HandleFunc("GET /api/v1/media/{media_id}", h.getByID)
	h.mux.HandleFunc("DELETE /api/v1/media/{media_id}", h.deleteMedia)
	h.mux.HandleFunc("PATCH /api/v1/media/{media_id}/main", h.setMain)
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.Handle("GET /metrics", promhttp.Handler())
}

// upload handles multipart/form-data photo uploads.
//
// Form fields:
//
//	file     — the image file (required)
//	user_id  — owner's user ID (required)
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	userID, err := parseIntField(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id is required and must be a valid integer")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "field 'file' is required")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	input := service.UploadInput{
		UserID:           userID,
		OriginalFilename: header.Filename,
		MimeType:         mimeType,
		FileSize:         header.Size,
		Content:          file,
	}

	media, err := h.svc.Upload(r.Context(), input)
	if err != nil {
		h.m.UploadErrors.Inc()
		switch {
		case errors.Is(err, service.ErrInvalidMimeType):
			writeError(w, http.StatusUnsupportedMediaType, err.Error())
		case errors.Is(err, service.ErrTooManyPhotos):
			writeError(w, http.StatusConflict, err.Error())
		default:
			h.log.WithError(err).Error("upload failed")
			writeError(w, http.StatusInternalServerError, "upload failed")
		}
		return
	}

	h.m.UploadsTotal.Inc()
	writeJSON(w, http.StatusCreated, toMediaResponse(media))
}

func (h *Handler) listByUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathInt(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	medias, err := h.svc.GetByUserID(r.Context(), userID)
	if err != nil {
		h.log.WithError(err).Error("list user media failed")
		writeError(w, http.StatusInternalServerError, "failed to list media")
		return
	}

	resp := make([]mediaResponse, len(medias))
	for i, m := range medias {
		resp[i] = toMediaResponse(m)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	mediaID, err := parsePathInt(r, "media_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media_id")
		return
	}

	media, err := h.svc.GetByID(r.Context(), mediaID)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			writeError(w, http.StatusNotFound, "media not found")
			return
		}
		h.log.WithError(err).Error("get media failed")
		writeError(w, http.StatusInternalServerError, "failed to get media")
		return
	}

	writeJSON(w, http.StatusOK, toMediaResponse(media))
}

func (h *Handler) deleteMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := parsePathInt(r, "media_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media_id")
		return
	}

	callerUserID, err := parseQueryInt(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id query param is required")
		return
	}

	if err := h.svc.Delete(r.Context(), mediaID, callerUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrMediaNotFound):
			writeError(w, http.StatusNotFound, "media not found")
		case errors.Is(err, service.ErrForbidden):
			writeError(w, http.StatusForbidden, "access denied")
		default:
			h.log.WithError(err).Error("delete media failed")
			writeError(w, http.StatusInternalServerError, "failed to delete media")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setMain(w http.ResponseWriter, r *http.Request) {
	mediaID, err := parsePathInt(r, "media_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid media_id")
		return
	}

	var body struct {
		UserID int64 `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == 0 {
		writeError(w, http.StatusBadRequest, "user_id is required in request body")
		return
	}

	if err := h.svc.SetMain(r.Context(), mediaID, body.UserID); err != nil {
		switch {
		case errors.Is(err, service.ErrMediaNotFound):
			writeError(w, http.StatusNotFound, "media not found")
		case errors.Is(err, service.ErrForbidden):
			writeError(w, http.StatusForbidden, "access denied")
		default:
			h.log.WithError(err).Error("set main photo failed")
			writeError(w, http.StatusInternalServerError, "failed to set main photo")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- middleware ---

func (h *Handler) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.status)

		h.m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		h.m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)

		h.log.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     rw.status,
			"latency_ms": time.Since(start).Milliseconds(),
		}).Info("http request")
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// --- response types ---

type mediaResponse struct {
	ID               int64  `json:"id"`
	UserID           int64  `json:"user_id"`
	URL              string `json:"url"`
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	FileSize         int64  `json:"file_size"`
	IsMain           bool   `json:"is_main"`
	UploadedAt       string `json:"uploaded_at"`
}

func toMediaResponse(m *service.MediaWithURL) mediaResponse {
	return mediaResponse{
		ID:               m.ID,
		UserID:           m.UserID,
		URL:              m.URL,
		OriginalFilename: m.OriginalFilename,
		MimeType:         m.MimeType,
		FileSize:         m.FileSize,
		IsMain:           m.IsMain,
		UploadedAt:       m.UploadedAt.Format(time.RFC3339),
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func parsePathInt(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}

func parseQueryInt(r *http.Request, key string) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, fmt.Errorf("missing query param %q", key)
	}
	return strconv.ParseInt(v, 10, 64)
}

func parseIntField(r *http.Request, key string) (int64, error) {
	v := r.FormValue(key)
	if v == "" {
		return 0, fmt.Errorf("missing form field %q", key)
	}
	return strconv.ParseInt(v, 10, 64)
}
