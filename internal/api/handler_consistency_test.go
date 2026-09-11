package api_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"sk5proxy/internal/api"
	"sk5proxy/internal/config"
	"sk5proxy/internal/upstream"
)

func TestHandler_UpsertUpstream_doesNotPersistConfigSelectorCannotPrepare(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	body := `{"id":"bad","name":"Bad","type":"socks5","address":"missing-port"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Upstreams) != 2 {
		t.Fatalf("persisted upstream count = %d, want 2", len(cfg.Upstreams))
	}
}

func TestHandler_UpsertUpstream_leavesSelectorUnchangedWhenSaveFails(t *testing.T) {
	// Given
	initial := config.Config{ActiveID: "first", Upstreams: []config.Upstream{{
		ID: "first", Name: "First", Type: config.TypeHTTP, Address: "127.0.0.1:8080",
	}}}
	selector, err := upstream.NewSelector(initial.Upstreams, initial.ActiveID)
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	before := selector.Config()
	store := config.NewStore(filepath.Join(t.TempDir(), "missing", "config.json"))
	handler := api.NewHandler(store, selector, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"id":"second","name":"Second","type":"http","address":"127.0.0.1:9090"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	cancelled, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(cancelled)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if got := selector.Config(); !reflect.DeepEqual(got, before) {
		t.Fatalf("selector config = %#v, want unchanged %#v", got, before)
	}
}
