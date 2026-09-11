package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type UpstreamType string
type ListenerType string

const (
	TypeSOCKS5     UpstreamType = "socks5"
	TypeHTTP       UpstreamType = "http"
	ListenerSOCKS5 ListenerType = "socks5"
	ListenerHTTP   ListenerType = "http"
)

var (
	ErrInvalid   = errors.New("invalid config")
	ErrInvalidID = errors.New("invalid upstream ID")
	idPattern    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

type Upstream struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     UpstreamType `json:"type"`
	Address  string       `json:"address"`
	Username string       `json:"username,omitempty"`
	Password string       `json:"password,omitempty"`
}

type Config struct {
	ActiveID  string     `json:"activeId"`
	Upstreams []Upstream `json:"upstreams"`
	Listeners []Listener `json:"listeners"`
}

type Listener struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Type       ListenerType `json:"type"`
	Address    string       `json:"address"`
	UpstreamID string       `json:"upstreamId"`
	Enabled    bool         `json:"enabled"`
}

type PublicUpstream struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     UpstreamType `json:"type"`
	Address  string       `json:"address"`
	Username string       `json:"username,omitempty"`
}

type PublicConfig struct {
	ActiveID  string           `json:"activeId"`
	Upstreams []PublicUpstream `json:"upstreams"`
	Listeners []Listener       `json:"listeners,omitempty"`
}

func (c Config) Public() PublicConfig {
	upstreams := make([]PublicUpstream, len(c.Upstreams))
	for i, item := range c.Upstreams {
		upstreams[i] = PublicUpstream{ID: item.ID, Name: item.Name, Type: item.Type, Address: item.Address, Username: item.Username}
	}
	return PublicConfig{ActiveID: c.ActiveID, Upstreams: upstreams, Listeners: append([]Listener(nil), c.Listeners...)}
}

type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", s.path, err)
	}
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", ErrInvalid)
	}
	if err := ensureEOF(decoder); err != nil {
		return Config{}, err
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s *Store) Save(ctx context.Context, cfg Config) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			err = errors.Join(err, tmp.Close())
		}
		if !renamed {
			removeErr := os.Remove(tmpName)
			if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, removeErr)
			}
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("protect temporary config: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	closed = true
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	renamed = true
	return nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing config data: %w", ErrInvalid)
	}
	return nil
}

func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("upstream ID %q: %w", id, ErrInvalidID)
	}
	return nil
}

func validateListener(item Listener, upstreamIDs map[string]struct{}) error {
	if err := ValidateID(item.ID); err != nil {
		return fmt.Errorf("listener: %w: %w", err, ErrInvalid)
	}
	if item.Name == "" || strings.TrimSpace(item.Name) != item.Name {
		return fmt.Errorf("listener %q name: %w", item.ID, ErrInvalid)
	}
	if item.Address == "" || strings.TrimSpace(item.Address) != item.Address {
		return fmt.Errorf("listener %q address: %w", item.ID, ErrInvalid)
	}
	if _, _, err := net.SplitHostPort(item.Address); err != nil {
		return fmt.Errorf("listener %q address: %w: %w", item.ID, err, ErrInvalid)
	}
	if item.Type != ListenerSOCKS5 && item.Type != ListenerHTTP {
		return fmt.Errorf("listener %q type: %w", item.ID, ErrInvalid)
	}
	if item.UpstreamID != "" {
		if _, exists := upstreamIDs[item.UpstreamID]; !exists {
			return fmt.Errorf("listener %q upstream %q: %w", item.ID, item.UpstreamID, ErrInvalid)
		}
	}
	return nil
}

func Validate(cfg Config) error {
	ids := make(map[string]struct{}, len(cfg.Upstreams))
	for _, item := range cfg.Upstreams {
		if err := ValidateID(item.ID); err != nil {
			return fmt.Errorf("%w: %w", err, ErrInvalid)
		}
		if item.Name == "" || strings.TrimSpace(item.Name) != item.Name {
			return fmt.Errorf("upstream %q name: %w", item.ID, ErrInvalid)
		}
		if item.Address == "" || strings.TrimSpace(item.Address) != item.Address {
			return fmt.Errorf("upstream %q address: %w", item.ID, ErrInvalid)
		}
		if _, _, err := net.SplitHostPort(item.Address); err != nil {
			return fmt.Errorf("upstream %q address: %w: %w", item.ID, err, ErrInvalid)
		}
		if item.Type != TypeSOCKS5 && item.Type != TypeHTTP {
			return fmt.Errorf("upstream %q: %w", item.ID, ErrInvalid)
		}
		if (item.Username == "") != (item.Password == "") {
			return fmt.Errorf("upstream %q credentials: %w", item.ID, ErrInvalid)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("duplicate upstream %q: %w", item.ID, ErrInvalid)
		}
		ids[item.ID] = struct{}{}
	}
	if cfg.ActiveID != "" {
		if _, exists := ids[cfg.ActiveID]; !exists {
			return fmt.Errorf("active upstream %q: %w", cfg.ActiveID, ErrInvalid)
		}
	}
	listenerIDs := make(map[string]struct{}, len(cfg.Listeners))
	addresses := make(map[string]struct{}, len(cfg.Listeners))
	for _, item := range cfg.Listeners {
		if err := validateListener(item, ids); err != nil {
			return err
		}
		if _, exists := listenerIDs[item.ID]; exists {
			return fmt.Errorf("duplicate listener %q: %w", item.ID, ErrInvalid)
		}
		listenerIDs[item.ID] = struct{}{}
		if item.Enabled {
			if _, exists := addresses[item.Address]; exists {
				return fmt.Errorf("duplicate enabled listener address %q: %w", item.Address, ErrInvalid)
			}
			addresses[item.Address] = struct{}{}
		}
	}
	return nil
}
