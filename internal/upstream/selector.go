package upstream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"

	"sk5proxy/internal/config"
)

var (
	ErrNotFound = errors.New("upstream not found")
	ErrNoActive = errors.New("no active upstream")
)

type entry struct {
	config config.Upstream
	dialer Dialer
}

type Selector struct {
	mu       sync.RWMutex
	entries  map[string]entry
	activeID string
}

type Prepared struct {
	entries  map[string]entry
	activeID string
}

func NewSelector(items []config.Upstream, activeID string) (*Selector, error) {
	prepared, err := Prepare(config.Config{ActiveID: activeID, Upstreams: items})
	if err != nil {
		return nil, err
	}
	return &Selector{entries: prepared.entries, activeID: prepared.activeID}, nil
}

func Prepare(cfg config.Config) (Prepared, error) {
	if err := config.Validate(cfg); err != nil {
		return Prepared{}, fmt.Errorf("validate selector config: %w", err)
	}
	prepared := Prepared{entries: make(map[string]entry, len(cfg.Upstreams)), activeID: cfg.ActiveID}
	for _, item := range cfg.Upstreams {
		dialer, err := NewDialer(item)
		if err != nil {
			return Prepared{}, fmt.Errorf("prepare upstream %q: %w", item.ID, err)
		}
		prepared.entries[item.ID] = entry{config: item, dialer: dialer}
	}
	return prepared, nil
}

func (s *Selector) Install(prepared Prepared) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = prepared.entries
	s.activeID = prepared.activeID
}

func (s *Selector) List() config.PublicConfig {
	return s.Config().Public()
}

func (s *Selector) Config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]config.Upstream, 0, len(s.entries))
	for _, item := range s.entries {
		items = append(items, item.config)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return config.Config{ActiveID: s.activeID, Upstreams: items}
}

func (s *Selector) Upsert(item config.Upstream) error {
	dialer, err := NewDialer(item)
	if err != nil {
		return fmt.Errorf("prepare upstream %q: %w", item.ID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[item.ID] = entry{config: item, dialer: dialer}
	return nil
}

func (s *Selector) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[id]; !exists {
		return fmt.Errorf("delete %q: %w", id, ErrNotFound)
	}
	delete(s.entries, id)
	if s.activeID == id {
		s.activeID = ""
	}
	return nil
}

func (s *Selector) Activate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[id]; !exists {
		return fmt.Errorf("activate %q: %w", id, ErrNotFound)
	}
	s.activeID = id
	return nil
}

func (s *Selector) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	s.mu.RLock()
	activeID := s.activeID
	s.mu.RUnlock()
	return s.DialContextFor(ctx, activeID, network, address)
}

func (s *Selector) DialContextFor(ctx context.Context, id, network, address string) (net.Conn, error) {
	s.mu.RLock()
	selected, exists := s.entries[id]
	s.mu.RUnlock()
	if !exists {
		return nil, ErrNoActive
	}
	conn, err := selected.dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("dial via upstream %q: %w", selected.config.ID, err)
	}
	return conn, nil
}
