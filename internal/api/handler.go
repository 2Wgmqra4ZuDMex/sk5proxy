package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"sk5proxy/internal/config"
	listener "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
)

var ErrInvalidJSON = errors.New("invalid JSON")

type Handler struct {
	store     *config.Store
	selector  *upstream.Selector
	listeners *listener.Manager
	logger    *slog.Logger
	mu        sync.Mutex
}

func NewHandler(store *config.Store, selector *upstream.Selector, logger *slog.Logger) *Handler {
	return &Handler{store: store, selector: selector, logger: logger}
}

func NewHandlerWithListeners(store *config.Store, selector *upstream.Selector, listeners *listener.Manager, logger *slog.Logger) *Handler {
	return &Handler{store: store, selector: selector, listeners: listeners, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/api/config")
	switch path {
	case "", "/":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, h.currentConfig().Public())
	case "/upstreams":
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.upsertUpstream(w, r)
	case "/activate":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.activateUpstream(w, r)
	default:
		h.routeResource(w, r, path)
	}
}

func (h *Handler) routeResource(w http.ResponseWriter, r *http.Request, path string) {
	prefix := "/upstreams/"
	isListener := strings.HasPrefix(path, "/listeners/")
	if isListener {
		prefix = "/listeners/"
	} else if !strings.HasPrefix(path, prefix) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	escapedID := strings.TrimPrefix(path, prefix)
	isSwitch := isListener && strings.HasSuffix(escapedID, "/switch")
	if isSwitch {
		escapedID = strings.TrimSuffix(escapedID, "/switch")
	}
	if escapedID == "" || strings.Contains(escapedID, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id, err := url.PathUnescape(escapedID)
	if err != nil || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if isSwitch && r.Method == http.MethodPost {
		h.switchListener(w, r, id)
		return
	}
	if isListener && r.Method == http.MethodPut {
		h.upsertListener(w, r, id)
		return
	}
	if r.Method != http.MethodDelete || isSwitch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := config.ValidateID(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid resource ID")
		return
	}
	if isListener {
		h.deleteListener(w, r, id)
		return
	}
	h.deleteUpstream(w, r, id)
}

func (h *Handler) currentConfig() config.Config {
	cfg := h.selector.Config()
	if h.listeners != nil {
		cfg.Listeners = h.listeners.Config()
	}
	return cfg
}

func decodeJSON(body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w: %w", err, ErrInvalidJSON)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing request data: %w", ErrInvalidJSON)
	}
	return nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
