package config_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"sk5proxy/internal/config"
)

func TestStore_Save_rejectsUnsafeUpstreamIDs(t *testing.T) {
	tests := []string{
		" leading",
		"trailing ",
		"nested/path",
		`quoted"id`,
		"percent%id",
		strings.Repeat("a", 65),
	}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			// Given
			store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
			cfg := config.Config{Upstreams: []config.Upstream{{
				ID: id, Name: "Proxy", Type: config.TypeHTTP, Address: "127.0.0.1:8080",
			}}}

			// When
			err := store.Save(context.Background(), cfg)

			// Then
			if !errors.Is(err, config.ErrInvalid) {
				t.Fatalf("Save() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestStore_Save_acceptsSafeUpstreamIDBoundary(t *testing.T) {
	// Given
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	id := strings.Repeat("a", 61) + "._-"
	cfg := config.Config{ActiveID: id, Upstreams: []config.Upstream{{
		ID: id, Name: "Proxy", Type: config.TypeHTTP, Address: "127.0.0.1:8080",
	}}}

	// When
	err := store.Save(context.Background(), cfg)

	// Then
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestStore_Save_rejectsAddressSelectorCannotPrepare(t *testing.T) {
	// Given
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Config{Upstreams: []config.Upstream{{
		ID: "proxy", Name: "Proxy", Type: config.TypeSOCKS5, Address: "missing-port",
	}}}

	// When
	err := store.Save(context.Background(), cfg)

	// Then
	if !errors.Is(err, config.ErrInvalid) {
		t.Fatalf("Save() error = %v, want ErrInvalid", err)
	}
}

func TestStore_Save_rejectsPartialCredentials(t *testing.T) {
	// Given
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	cfg := config.Config{Upstreams: []config.Upstream{{
		ID: "proxy", Name: "Proxy", Type: config.TypeHTTP, Address: "127.0.0.1:8080", Username: "alice",
	}}}

	// When
	err := store.Save(context.Background(), cfg)

	// Then
	if !errors.Is(err, config.ErrInvalid) {
		t.Fatalf("Save() error = %v, want ErrInvalid", err)
	}
}
