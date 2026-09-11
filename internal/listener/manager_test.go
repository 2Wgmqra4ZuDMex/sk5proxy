package listener_test

import (
	"context"
	"net"
	"testing"

	"sk5proxy/internal/config"
	listener "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
)

func TestManager_Prepare_rejectsOccupiedPort_withoutChangingRuntime(t *testing.T) {
	// Given
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer occupied.Close()
	selector, err := upstream.NewSelector(nil, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	cfg := config.Config{Listeners: []config.Listener{{
		ID: "blocked", Name: "Blocked", Type: config.ListenerSOCKS5,
		Address: occupied.Addr().String(), Enabled: true,
	}}}

	// When
	_, err = manager.Prepare(cfg)

	// Then
	if err == nil {
		t.Fatal("Prepare() error = nil, want bind error")
	}
	if len(manager.List()) != 0 {
		t.Fatalf("runtime listeners = %#v, want empty", manager.List())
	}
}

func TestManager_Commit_switchesOneRoute_withoutChangingAnother(t *testing.T) {
	// Given
	selector, err := upstream.NewSelector([]config.Upstream{
		{ID: "a", Name: "A", Type: config.TypeHTTP, Address: "127.0.0.1:10001"},
		{ID: "b", Name: "B", Type: config.TypeHTTP, Address: "127.0.0.1:10002"},
	}, "a")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	initial := config.Config{Upstreams: selector.Config().Upstreams, Listeners: []config.Listener{
		{ID: "left", Name: "Left", Type: config.ListenerHTTP, Address: "127.0.0.1:0", UpstreamID: "a", Enabled: true},
		{ID: "right", Name: "Right", Type: config.ListenerSOCKS5, Address: "localhost:0", UpstreamID: "b", Enabled: true},
	}}
	prepared, err := manager.Prepare(initial)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	next := initial
	next.Listeners = append([]config.Listener(nil), initial.Listeners...)
	next.Listeners[0].UpstreamID = "b"

	// When
	prepared, err = manager.Prepare(next)
	if err != nil {
		t.Fatalf("Prepare(switch) error = %v", err)
	}
	manager.Commit(prepared)

	// Then
	got := manager.Config()
	if got[0].UpstreamID != "b" || got[1].UpstreamID != "b" {
		t.Fatalf("routes = %#v", got)
	}
}
