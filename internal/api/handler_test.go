package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"sk5proxy/internal/api"
	"sk5proxy/internal/config"
	"sk5proxy/internal/upstream"
)

func setupTest(t *testing.T) (*api.Handler, *config.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	store := config.NewStore(path)
	initial := config.Config{
		ActiveID: "upstream1",
		Upstreams: []config.Upstream{
			{ID: "upstream1", Name: "First", Type: config.TypeHTTP, Address: "127.0.0.1:8080", Username: "user1", Password: "pass1"},
			{ID: "upstream2", Name: "Second", Type: config.TypeSOCKS5, Address: "127.0.0.1:1080", Username: "user2", Password: "pass2"},
		},
	}
	if err := store.Save(context.Background(), initial); err != nil {
		t.Fatalf("save initial config: %v", err)
	}
	selector, err := upstream.NewSelector(initial.Upstreams, initial.ActiveID)
	if err != nil {
		t.Fatalf("create selector: %v", err)
	}
	handler := api.NewHandler(store, selector, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return handler, store, path
}

func TestHandler_GetConfig_returnsPublicConfigWithoutPasswords(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var result config.PublicConfig
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.ActiveID != "upstream1" {
		t.Errorf("activeId = %q, want %q", result.ActiveID, "upstream1")
	}
	if len(result.Upstreams) != 2 {
		t.Fatalf("upstreams count = %d, want 2", len(result.Upstreams))
	}
	encoded := w.Body.String()
	if bytes.Contains([]byte(encoded), []byte("pass1")) || bytes.Contains([]byte(encoded), []byte("pass2")) {
		t.Error("response contains passwords")
	}
}

func TestHandler_UpsertUpstream_createsNewUpstream(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	body := `{"id":"upstream3","name":"Third","type":"http","address":"127.0.0.1:9090","username":"user3","password":"pass3"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Upstreams) != 3 {
		t.Errorf("upstreams count = %d, want 3", len(cfg.Upstreams))
	}
	found := false
	for _, u := range cfg.Upstreams {
		if u.ID == "upstream3" {
			found = true
			if u.Password != "pass3" {
				t.Errorf("password = %q, want %q", u.Password, "pass3")
			}
		}
	}
	if !found {
		t.Error("upstream3 not found in config")
	}
}

func TestHandler_UpsertUpstream_preservesPasswordWhenOmitted(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	body := `{"id":"upstream1","name":"First Updated","type":"http","address":"127.0.0.1:8081","username":"user1-updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	for _, u := range cfg.Upstreams {
		if u.ID == "upstream1" {
			if u.Password != "pass1" {
				t.Errorf("password = %q, want %q (preserved)", u.Password, "pass1")
			}
			if u.Name != "First Updated" {
				t.Errorf("name = %q, want %q", u.Name, "First Updated")
			}
			return
		}
	}
	t.Error("upstream1 not found")
}

func TestHandler_DeleteUpstream_removesInactiveUpstream(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/upstream2", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Upstreams) != 1 {
		t.Errorf("upstreams count = %d, want 1", len(cfg.Upstreams))
	}
	for _, u := range cfg.Upstreams {
		if u.ID == "upstream2" {
			t.Error("upstream2 should have been deleted")
		}
	}
}

func TestHandler_DeleteUpstream_rejectsActiveUpstream(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/config/upstreams/upstream1", nil)
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestHandler_ActivateUpstream_changesActiveID(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	body := `{"id":"upstream2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/activate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ActiveID != "upstream2" {
		t.Errorf("activeId = %q, want %q", cfg.ActiveID, "upstream2")
	}
}

func TestHandler_ActivateUpstream_rejectsNonexistentUpstream(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	body := `{"id":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/config/activate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_UpsertUpstream_validatesRequiredFields(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	body := `{"id":"","name":"","type":"http","address":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_UpsertUpstream_validatesType(t *testing.T) {
	// Given
	handler, _, _ := setupTest(t)
	body := `{"id":"test","name":"Test","type":"invalid","address":"127.0.0.1:8080"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_UpsertUpstream_editPreservesIDAndPersistsAllFields(t *testing.T) {
	// Given
	handler, store, _ := setupTest(t)
	body := `{"id":"upstream1","name":"Renamed","type":"socks5","address":"10.0.0.1:1080","username":"newuser","password":"newpass"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/upstreams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// When
	handler.ServeHTTP(w, req)

	// Then
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	cfg, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Upstreams) != 2 {
		t.Fatalf("upstreams count = %d, want 2 (edit must not add)", len(cfg.Upstreams))
	}
	for _, u := range cfg.Upstreams {
		if u.ID == "upstream1" {
			if u.Name != "Renamed" {
				t.Errorf("name = %q, want %q", u.Name, "Renamed")
			}
			if u.Type != config.TypeSOCKS5 {
				t.Errorf("type = %q, want %q", u.Type, config.TypeSOCKS5)
			}
			if u.Address != "10.0.0.1:1080" {
				t.Errorf("address = %q, want %q", u.Address, "10.0.0.1:1080")
			}
			if u.Username != "newuser" {
				t.Errorf("username = %q, want %q", u.Username, "newuser")
			}
			if u.Password != "newpass" {
				t.Errorf("password = %q, want %q", u.Password, "newpass")
			}
			return
		}
	}
	t.Error("upstream1 not found after edit")
}
