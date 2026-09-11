package api

import (
	"net/http"

	"sk5proxy/internal/config"
)

type listenerRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Address    string `json:"address"`
	UpstreamID string `json:"upstreamId"`
	Enabled    bool   `json:"enabled"`
}

type switchRequest struct {
	UpstreamID string `json:"upstreamId"`
}

func (h *Handler) upsertListener(w http.ResponseWriter, r *http.Request, id string) {
	var req listenerRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Type != string(config.ListenerSOCKS5) && req.Type != string(config.ListenerHTTP) {
		writeError(w, http.StatusBadRequest, "type must be socks5 or http")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	item := config.Listener{ID: id, Name: req.Name, Type: config.ListenerType(req.Type), Address: req.Address, UpstreamID: req.UpstreamID, Enabled: req.Enabled}
	current.Listeners = replaceListener(current.Listeners, item)
	h.persistAndInstall(w, r, mutation{config: current, operation: "listener.upsert", resourceID: id})
}

func (h *Handler) switchListener(w http.ResponseWriter, r *http.Request, id string) {
	var req switchRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	found := false
	for index := range current.Listeners {
		if current.Listeners[index].ID == id {
			current.Listeners[index].UpstreamID = req.UpstreamID
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "listener not found")
		return
	}
	h.persistAndInstall(w, r, mutation{config: current, operation: "listener.switch", resourceID: id})
}

func (h *Handler) deleteListener(w http.ResponseWriter, r *http.Request, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	current := h.currentConfig()
	listeners := make([]config.Listener, 0, len(current.Listeners))
	found := false
	for _, item := range current.Listeners {
		if item.ID == id {
			found = true
			continue
		}
		listeners = append(listeners, item)
	}
	if !found {
		writeError(w, http.StatusNotFound, "listener not found")
		return
	}
	current.Listeners = listeners
	h.persistAndInstall(w, r, mutation{config: current, operation: "listener.delete", resourceID: id})
}

func replaceListener(items []config.Listener, replacement config.Listener) []config.Listener {
	result := append([]config.Listener(nil), items...)
	for index := range result {
		if result[index].ID == replacement.ID {
			result[index] = replacement
			return result
		}
	}
	return append(result, replacement)
}
