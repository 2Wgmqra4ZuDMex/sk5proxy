package config_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sk5proxy/internal/config"
)

func TestStore_SaveThenLoad_preservesCredentials(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "config.json")
	want := config.Config{ActiveID: "office", Upstreams: []config.Upstream{{
		ID: "office", Name: "Office", Type: config.TypeSOCKS5, Address: "127.0.0.1:1080",
		Username: "alice", Password: "secret",
	}}}
	store := config.NewStore(path)

	// When
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load(context.Background())

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestConfig_Public_redactsPasswords(t *testing.T) {
	// Given
	cfg := config.Config{ActiveID: "corp", Upstreams: []config.Upstream{{
		ID: "corp", Name: "Corporate", Type: config.TypeHTTP, Address: "proxy.local:3128",
		Username: "bob", Password: "top-secret",
	}}}

	// When
	public := cfg.Public()
	encoded, err := json.Marshal(public)

	// Then
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(encoded) != `{"activeId":"corp","upstreams":[{"id":"corp","name":"Corporate","type":"http","address":"proxy.local:3128","username":"bob"}]}` {
		t.Fatalf("public JSON = %s", encoded)
	}
	if cfg.Upstreams[0].Password != "top-secret" {
		t.Fatal("Public() mutated stored credentials")
	}
}

func TestStore_Load_rejectsUnknownFields(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"activeId":"","upstreams":[],"surprise":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// When
	_, err := config.NewStore(path).Load(context.Background())

	// Then
	if err == nil {
		t.Fatal("Load() error = nil, want malformed config error")
	}
}
