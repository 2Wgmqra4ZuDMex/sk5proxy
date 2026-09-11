package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sk5proxy/internal/config"
)

func TestLoadConfig_createsEmptyConfigWhenFileIsAbsent(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	// When
	defaults := defaultListeners("127.0.0.1:1080", "127.0.0.1:8080", "")
	store, got, err := loadConfig(context.Background(), path, defaults)

	// Then
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	want := config.Config{Upstreams: []config.Upstream{}, Listeners: defaults}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("config = %#v, want %#v", got, want)
	}
	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() created config error = %v", err)
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("created config = %#v, want %#v", loaded, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadConfig_preservesExistingConfig(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "config.json")
	want := config.Config{ActiveID: "existing", Upstreams: []config.Upstream{{
		ID: "existing", Name: "Existing", Type: config.TypeSOCKS5, Address: "proxy:1080",
	}}}
	store := config.NewStore(path)
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// When
	defaults := defaultListeners("127.0.0.1:1180", "127.0.0.1:8180", want.ActiveID)
	_, got, err := loadConfig(context.Background(), path, defaults)

	// Then
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(got.Listeners) != 2 || got.Listeners[0].UpstreamID != want.ActiveID || got.Listeners[1].UpstreamID != want.ActiveID {
		t.Fatalf("migrated listeners = %#v", got.Listeners)
	}
	if got.ActiveID != want.ActiveID || !reflect.DeepEqual(got.Upstreams, want.Upstreams) {
		t.Fatalf("migrated config = %#v, legacy = %#v", got, want)
	}
	persisted, err := config.NewStore(path).Load(context.Background())
	if err != nil || !reflect.DeepEqual(persisted, got) {
		t.Fatalf("persisted migration = %#v, %v", persisted, err)
	}
}

func TestRuntimeConfig_defaultsProxyListenersToLoopback(t *testing.T) {
	// Given
	env := func(string) (string, bool) { return "", false }

	// When
	got, err := runtimeConfigFromEnv(env)

	// Then
	if err != nil {
		t.Fatalf("runtimeConfigFromEnv() error = %v", err)
	}
	if got.SOCKSAddr != "127.0.0.1:1080" || got.HTTPAddr != "127.0.0.1:8080" || got.WebAddr != "127.0.0.1:8081" {
		t.Fatalf("addresses = %#v", got)
	}
}

func TestRuntimeConfig_allowsExplicitWildcardProxyListeners(t *testing.T) {
	// Given
	values := map[string]string{
		"SK5_SOCKS_ADDR": "0.0.0.0:1080",
		"SK5_HTTP_ADDR":  "0.0.0.0:8080",
		"SK5_WEB_ADDR":   "0.0.0.0:8081",
	}
	env := func(key string) (string, bool) { value, ok := values[key]; return value, ok }

	// When
	got, err := runtimeConfigFromEnv(env)

	// Then
	if err != nil {
		t.Fatalf("runtimeConfigFromEnv() error = %v", err)
	}
	if got.SOCKSAddr != values["SK5_SOCKS_ADDR"] || got.HTTPAddr != values["SK5_HTTP_ADDR"] || got.WebAddr != values["SK5_WEB_ADDR"] {
		t.Fatalf("addresses = %#v", got)
	}
}

func TestRuntimeConfig_rejectsWildcardProxyDefaultWithoutExplicitEnv(t *testing.T) {
	// Given
	env := func(key string) (string, bool) {
		if key == "SK5_SOCKS_ADDR" {
			return "", true
		}
		return "", false
	}

	// When
	_, err := runtimeConfigFromEnv(env)

	// Then
	if err == nil {
		t.Fatal("runtimeConfigFromEnv() error = nil")
	}
}
