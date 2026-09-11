package api_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"sk5proxy/internal/api"
	"sk5proxy/internal/config"
	listener "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
)

func TestHandler_SwitchListener_changesOnlyRequestedRoute(t *testing.T) {
	// Given
	cfg := config.Config{Upstreams: []config.Upstream{
		{ID: "a", Name: "A", Type: config.TypeHTTP, Address: "127.0.0.1:10001"},
		{ID: "b", Name: "B", Type: config.TypeHTTP, Address: "127.0.0.1:10002"},
	}, Listeners: []config.Listener{
		{ID: "left", Name: "Left", Type: config.ListenerHTTP, Address: "127.0.0.1:0", UpstreamID: "a", Enabled: false},
		{ID: "right", Name: "Right", Type: config.ListenerSOCKS5, Address: "127.0.0.1:0", UpstreamID: "a", Enabled: false},
	}}
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(context.Background(), cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	selector, err := upstream.NewSelector(cfg.Upstreams, cfg.ActiveID)
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	prepared, err := manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	handler := api.NewHandlerWithListeners(store, selector, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/api/config/listeners/left/switch", bytes.NewBufferString(`{"upstreamId":"b"}`))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	stored, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if stored.Listeners[0].UpstreamID != "b" || stored.Listeners[1].UpstreamID != "a" {
		t.Fatalf("listeners = %#v", stored.Listeners)
	}
}

func TestHandler_DeleteUpstream_rejectsListenerReference(t *testing.T) {
	// Given
	cfg := config.Config{Upstreams: []config.Upstream{{ID: "upstream2", Name: "Second", Type: config.TypeHTTP, Address: "127.0.0.1:10002"}}, Listeners: []config.Listener{{ID: "route", Name: "Route", Type: config.ListenerHTTP, Address: "127.0.0.1:0", UpstreamID: "upstream2", Enabled: false}}}
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(context.Background(), cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	selector, err := upstream.NewSelector(cfg.Upstreams, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	prepared, err := manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	handler := api.NewHandlerWithListeners(store, selector, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/upstream2", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestHandler_UpsertListener_releasesPreparedPort_whenSaveFails(t *testing.T) {
	// Given
	selector, err := upstream.NewSelector(nil, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	handler := api.NewHandlerWithListeners(store, selector, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	body := `{"name":"Temporary","type":"http","address":"` + address + `","upstreamId":"","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/listeners/temporary", bytes.NewBufferString(body))
	cancelled, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(cancelled)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	rebound, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("prepared port remained bound: %v", err)
	}
	if err := rebound.Close(); err != nil {
		t.Fatalf("Close() rebound error = %v", err)
	}
	if len(manager.Config()) != 0 {
		t.Fatalf("runtime config = %#v, want unchanged", manager.Config())
	}
}

func TestHandler_UpsertListener_doesNotPersist_whenPortIsOccupied(t *testing.T) {
	// Given
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer occupied.Close()
	cfg := config.Config{Upstreams: []config.Upstream{}, Listeners: []config.Listener{}}
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(context.Background(), cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	selector, err := upstream.NewSelector(nil, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	prepared, err := manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	handler := api.NewHandlerWithListeners(store, selector, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"name":"Blocked","type":"socks5","address":"` + occupied.Addr().String() + `","upstreamId":"","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/listeners/blocked", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	stored, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(stored.Listeners) != 0 || len(manager.Config()) != 0 {
		t.Fatalf("disk/runtime changed: %#v / %#v", stored.Listeners, manager.Config())
	}
}
