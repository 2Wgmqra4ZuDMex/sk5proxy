package api

import (
	"errors"
	"net/http"

	"sk5proxy/internal/config"
	listener "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
)

type upstreamRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) upsertUpstream(w http.ResponseWriter, r *http.Request) {
	var req upstreamRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Type != string(config.TypeSOCKS5) && req.Type != string(config.TypeHTTP) {
		writeError(w, http.StatusBadRequest, "type must be socks5 or http")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	upstreams := append([]config.Upstream(nil), current.Upstreams...)
	item := config.Upstream{
		ID: req.ID, Name: req.Name, Type: config.UpstreamType(req.Type), Address: req.Address,
		Username: req.Username, Password: req.Password,
	}
	found := false
	for i, existing := range upstreams {
		if existing.ID != req.ID {
			continue
		}
		found = true
		if item.Username == "" {
			item.Password = ""
		} else if item.Password == "" {
			item.Password = existing.Password
		}
		upstreams[i] = item
		break
	}
	if !found {
		upstreams = append(upstreams, item)
	}
	h.persistAndInstall(w, r, mutation{
		config: config.Config{ActiveID: current.ActiveID, Upstreams: upstreams, Listeners: current.Listeners}, operation: "upsert", resourceID: req.ID,
	})
}

func (h *Handler) deleteUpstream(w http.ResponseWriter, r *http.Request, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	if current.ActiveID == id {
		writeError(w, http.StatusConflict, "cannot delete active upstream")
		return
	}
	for _, item := range current.Listeners {
		if item.UpstreamID == id {
			writeError(w, http.StatusConflict, "cannot delete referenced upstream")
			return
		}
	}
	upstreams := make([]config.Upstream, 0, len(current.Upstreams))
	found := false
	for _, item := range current.Upstreams {
		if item.ID == id {
			found = true
			continue
		}
		upstreams = append(upstreams, item)
	}
	if !found {
		writeError(w, http.StatusNotFound, "upstream not found")
		return
	}
	h.persistAndInstall(w, r, mutation{
		config: config.Config{ActiveID: current.ActiveID, Upstreams: upstreams, Listeners: current.Listeners}, operation: "delete", resourceID: id,
	})
}

type activateRequest struct {
	ID string `json:"id"`
}

func (h *Handler) activateUpstream(w http.ResponseWriter, r *http.Request) {
	var req activateRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := config.ValidateID(req.ID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upstream ID")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	found := false
	for _, item := range current.Upstreams {
		if item.ID == req.ID {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "upstream not found")
		return
	}
	if h.persistAndInstall(w, r, mutation{
		config: config.Config{ActiveID: req.ID, Upstreams: current.Upstreams, Listeners: current.Listeners}, operation: "activate", resourceID: req.ID,
	}) {
		h.logger.Info("upstream.activated", "upstream_id", req.ID)
	}
}

type mutation struct {
	config     config.Config
	operation  string
	resourceID string
}

func (h *Handler) persistAndInstall(w http.ResponseWriter, r *http.Request, change mutation) bool {
	prepared, err := upstream.Prepare(change.config)
	if err != nil {
		if errors.Is(err, config.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "invalid upstream configuration")
			return false
		}
		h.logger.Error("selector.prepare_failed", "operation", change.operation, "resource_id", change.resourceID, "error", err)
		writeError(w, http.StatusInternalServerError, "prepare selector failed")
		return false
	}
	var preparedListeners listener.Prepared
	if h.listeners != nil {
		preparedListeners, err = h.listeners.Prepare(change.config)
		if err != nil {
			writeError(w, http.StatusConflict, "listener bind failed")
			return false
		}
	}
	if err := h.store.Save(r.Context(), change.config); err != nil {
		preparedListeners.Abort()
		h.logger.Error("config.save_failed", "operation", change.operation, "resource_id", change.resourceID, "error", err)
		writeError(w, http.StatusInternalServerError, "save failed")
		return false
	}
	h.selector.Install(prepared)
	if h.listeners != nil {
		h.listeners.Commit(preparedListeners)
	}
	writeJSON(w, http.StatusOK, change.config.Public())
	return true
}
