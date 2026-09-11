package listener

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"

	"sk5proxy/internal/config"
	"sk5proxy/internal/httpproxy"
	"sk5proxy/internal/socks5"
	"sk5proxy/internal/upstream"
)

var ErrClosed = errors.New("listener manager closed")

type routeDialer struct {
	mu         sync.RWMutex
	selector   *upstream.Selector
	upstreamID string
}

func (d *routeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.RLock()
	id := d.upstreamID
	d.mu.RUnlock()
	return d.selector.DialContextFor(ctx, id, network, address)
}

func (d *routeDialer) setUpstream(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.upstreamID = id
}

type running struct {
	mu           sync.RWMutex
	config       config.Listener
	listener     net.Listener
	httpListener *dispatchListener
	route        *routeDialer
	http         *httpproxy.Server
}

type Prepared struct {
	desired   []config.Listener
	upstreams []config.Upstream
	opened    map[string]*running
	reused    map[string]*running
}

type Manager struct {
	ctx       context.Context
	selector  *upstream.Selector
	mu        sync.RWMutex
	running   map[string]*running
	configs   []config.Listener
	upstreams []config.Upstream
	closed    bool
}

func NewManager(ctx context.Context, selector *upstream.Selector) *Manager {
	return &Manager{ctx: ctx, selector: selector, running: make(map[string]*running)}
}

func (m *Manager) Prepare(cfg config.Config) (Prepared, error) {
	if err := config.Validate(cfg); err != nil {
		return Prepared{}, fmt.Errorf("validate listeners: %w", err)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return Prepared{}, ErrClosed
	}
	prepared := Prepared{
		desired:   append([]config.Listener(nil), cfg.Listeners...),
		upstreams: append([]config.Upstream(nil), cfg.Upstreams...),
		opened:    make(map[string]*running),
		reused:    make(map[string]*running),
	}
	for _, item := range cfg.Listeners {
		if !item.Enabled {
			continue
		}
		if current, exists := m.running[item.ID]; exists && current.address() == item.Address {
			prepared.reused[item.ID] = current
			continue
		}
		bound, err := net.Listen("tcp", item.Address)
		if err != nil {
			prepared.Abort()
			return Prepared{}, fmt.Errorf("bind listener %q on %s: %w", item.ID, item.Address, err)
		}
		route := &routeDialer{selector: m.selector, upstreamID: item.UpstreamID}
		httpListener := newDispatchListener(bound.Addr())
		current := &running{
			config: item, listener: bound, httpListener: httpListener,
			route: route, http: httpproxy.NewServer(route),
		}
		prepared.opened[item.ID] = current
	}
	return prepared, nil
}

func (p *Prepared) Abort() {
	for _, item := range p.opened {
		_ = item.listener.Close()
	}
	p.opened = nil
}

func (m *Manager) Commit(prepared Prepared) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := make(map[string]*running, len(prepared.opened)+len(prepared.reused))
	upstreamsChanged := !reflect.DeepEqual(m.upstreams, prepared.upstreams)
	for _, item := range prepared.desired {
		if !item.Enabled {
			continue
		}
		if current, exists := prepared.reused[item.ID]; exists {
			current.mu.Lock()
			current.config = item
			current.mu.Unlock()
			routeChanged := current.route.upstream() != item.UpstreamID
			current.route.setUpstream(item.UpstreamID)
			if current.http != nil && (upstreamsChanged || routeChanged) {
				current.http.RoutingChanged()
			}
			next[item.ID] = current
			continue
		}
		current := prepared.opened[item.ID]
		next[item.ID] = current
		go current.serve(m.ctx)
	}
	for id, current := range m.running {
		if _, kept := next[id]; !kept {
			_ = current.listener.Close()
		}
	}
	m.running = next
	m.configs = append([]config.Listener(nil), prepared.desired...)
	m.upstreams = append([]config.Upstream(nil), prepared.upstreams...)
}

func (r *running) serve(ctx context.Context) {
	go func() { _ = r.http.Serve(ctx, r.httpListener) }()
	defer r.httpListener.Close()
	socks := socks5.NewServer(r.route)
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			return
		}
		switch r.protocol() {
		case config.ListenerSOCKS5:
			go func() { _ = socks.Handle(ctx, conn) }()
		case config.ListenerHTTP:
			if err := r.httpListener.deliver(ctx, conn); err != nil {
				_ = conn.Close()
			}
		}
	}
}

func (r *running) protocol() config.ListenerType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Type
}

func (r *running) address() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Address
}

func (r *running) currentConfig() config.Listener {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

func (d *routeDialer) upstream() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.upstreamID
}

func (m *Manager) List() []config.Listener {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]config.Listener, 0, len(m.running))
	for _, item := range m.running {
		items = append(items, item.currentConfig())
	}
	return items
}

func (m *Manager) Config() []config.Listener {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]config.Listener(nil), m.configs...)
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	var errs []error
	for _, item := range m.running {
		errs = append(errs, item.listener.Close())
	}
	m.running = make(map[string]*running)
	m.configs = nil
	m.upstreams = nil
	return errors.Join(errs...)
}
