package upstream_test

import (
	"context"
	"testing"

	"sk5proxy/internal/config"
	"sk5proxy/internal/upstream"
)

func TestSelector_Install_routesNewConnectionsWithoutClosingExisting(t *testing.T) {
	// Given
	first := startHTTPProxy(t, "", "")
	second := startHTTPProxy(t, "", "")
	selector, err := upstream.NewSelector([]config.Upstream{{
		ID: "active", Name: "First", Type: config.TypeHTTP, Address: first,
	}}, "active")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	oldConn, err := selector.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatalf("first DialContext() error = %v", err)
	}
	defer oldConn.Close()
	prepared, err := upstream.Prepare(config.Config{ActiveID: "active", Upstreams: []config.Upstream{{
		ID: "active", Name: "Second", Type: config.TypeHTTP, Address: second,
	}}})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	// When
	selector.Install(prepared)
	newConn, err := selector.DialContext(context.Background(), "tcp", "example.test:443")
	if err != nil {
		t.Fatalf("second DialContext() error = %v", err)
	}
	defer newConn.Close()

	// Then
	assertEcho(t, oldConn, "old")
	assertEcho(t, newConn, "new")
}
