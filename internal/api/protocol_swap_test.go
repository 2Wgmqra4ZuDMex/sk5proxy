package api_test

import (
	"bufio"
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

func TestHandler_UpsertListener_keepsHTTPAccepting_whenProtocolSwapSaveFails(t *testing.T) {
	// Given
	address := availableAddress(t)
	cfg := config.Config{Upstreams: []config.Upstream{{
		ID: "up", Name: "Up", Type: config.TypeHTTP, Address: "127.0.0.1:1",
	}}, Listeners: []config.Listener{{
		ID: "shared", Name: "Shared", Type: config.ListenerHTTP,
		Address: address, UpstreamID: "up", Enabled: true,
	}}}
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
	body := `{"name":"Shared","type":"socks5","address":"` + address + `","upstreamId":"up","enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/listeners/shared", bytes.NewBufferString(body))
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
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("HTTP listener stopped accepting: %v", err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "CONNECT target.test:443 HTTP/1.1\r\nHost: target.test:443\r\n\r\n"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("HTTP status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
}

func availableAddress(t *testing.T) string {
	t.Helper()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return address
}
