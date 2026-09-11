package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"sk5proxy/internal/config"
)

const (
	defaultConfigPath = "config.json"
	defaultSOCKSAddr  = "127.0.0.1:1080"
	defaultHTTPAddr   = "127.0.0.1:8080"
	defaultWebAddr    = "127.0.0.1:8081"
)

type RuntimeConfig struct {
	ConfigPath string
	SOCKSAddr  string
	HTTPAddr   string
	WebAddr    string
}

type envLookup func(string) (string, bool)

func runtimeConfigFromEnv(lookup envLookup) (RuntimeConfig, error) {
	values := RuntimeConfig{
		ConfigPath: envOrDefault(lookup, "SK5_CONFIG_PATH", defaultConfigPath),
		SOCKSAddr:  envOrDefault(lookup, "SK5_SOCKS_ADDR", defaultSOCKSAddr),
		HTTPAddr:   envOrDefault(lookup, "SK5_HTTP_ADDR", defaultHTTPAddr),
		WebAddr:    envOrDefault(lookup, "SK5_WEB_ADDR", defaultWebAddr),
	}
	for name, address := range map[string]string{
		"SK5_SOCKS_ADDR": values.SOCKSAddr,
		"SK5_HTTP_ADDR":  values.HTTPAddr,
		"SK5_WEB_ADDR":   values.WebAddr,
	} {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return RuntimeConfig{}, fmt.Errorf("parse %s: %w", name, err)
		}
	}
	return values, nil
}

func envOrDefault(lookup envLookup, name, fallback string) string {
	value, exists := lookup(name)
	if !exists {
		return fallback
	}
	return value
}

func defaultListeners(socksAddress, httpAddress, upstreamID string) []config.Listener {
	return []config.Listener{
		{ID: "default-socks5", Name: "Default SOCKS5", Type: config.ListenerSOCKS5, Address: socksAddress, UpstreamID: upstreamID, Enabled: true},
		{ID: "default-http", Name: "Default HTTP", Type: config.ListenerHTTP, Address: httpAddress, UpstreamID: upstreamID, Enabled: true},
	}
}

func loadConfig(ctx context.Context, path string, defaults []config.Listener) (*config.Store, config.Config, error) {
	store := config.NewStore(path)
	loaded, err := store.Load(ctx)
	if err == nil {
		if loaded.Listeners == nil {
			loaded.Listeners = append([]config.Listener(nil), defaults...)
			for index := range loaded.Listeners {
				if loaded.Listeners[index].UpstreamID == "" {
					loaded.Listeners[index].UpstreamID = loaded.ActiveID
				}
			}
			if err := store.Save(ctx, loaded); err != nil {
				return nil, config.Config{}, fmt.Errorf("persist listener migration: %w", err)
			}
		}
		return store, loaded, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, config.Config{}, err
	}
	empty := config.Config{Upstreams: []config.Upstream{}, Listeners: defaults}
	if err := store.Save(ctx, empty); err != nil {
		return nil, config.Config{}, fmt.Errorf("create empty config: %w", err)
	}
	return store, empty, nil
}
