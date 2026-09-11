package listener_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"

	"sk5proxy/internal/config"
	listener "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
)

func TestManager_HTTPListeners_routeIndependently_afterOneSwitch(t *testing.T) {
	// Given
	upstreamA := markerProxy(t, "A")
	upstreamB := markerProxy(t, "B")
	upstreamC := markerProxy(t, "C")
	addressA := freeAddress(t)
	addressB := freeAddress(t)
	cfg := config.Config{Upstreams: []config.Upstream{
		{ID: "a", Name: "A", Type: config.TypeHTTP, Address: upstreamA},
		{ID: "b", Name: "B", Type: config.TypeHTTP, Address: upstreamB},
		{ID: "c", Name: "C", Type: config.TypeHTTP, Address: upstreamC},
	}, Listeners: []config.Listener{
		{ID: "route-a", Name: "Route A", Type: config.ListenerHTTP, Address: addressA, UpstreamID: "a", Enabled: true},
		{ID: "route-b", Name: "Route B", Type: config.ListenerHTTP, Address: addressB, UpstreamID: "b", Enabled: true},
	}}
	selector, err := upstream.NewSelector(cfg.Upstreams, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	prepared, err := manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	if got := connectMarker(t, addressA); got != "A" {
		t.Fatalf("route A marker = %q", got)
	}
	if got := connectMarker(t, addressB); got != "B" {
		t.Fatalf("route B marker = %q", got)
	}
	cfg.Listeners[0].UpstreamID = "c"

	// When
	prepared, err = manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare(switch) error = %v", err)
	}
	manager.Commit(prepared)

	// Then
	if got := connectMarker(t, addressA); got != "C" {
		t.Fatalf("switched route A marker = %q", got)
	}
	if got := connectMarker(t, addressB); got != "B" {
		t.Fatalf("untouched route B marker = %q", got)
	}
}

func TestManager_Commit_swapsHTTPToSOCKS5_onSameAddress(t *testing.T) {
	// Given
	upstreamAddress := markerProxy(t, "A")
	address := freeAddress(t)
	cfg := config.Config{Upstreams: []config.Upstream{{
		ID: "a", Name: "A", Type: config.TypeHTTP, Address: upstreamAddress,
	}}, Listeners: []config.Listener{{
		ID: "shared", Name: "Shared", Type: config.ListenerHTTP,
		Address: address, UpstreamID: "a", Enabled: true,
	}}}
	selector, err := upstream.NewSelector(cfg.Upstreams, "")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	manager := listener.NewManager(context.Background(), selector)
	defer manager.Close()
	prepared, err := manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manager.Commit(prepared)
	if got := connectMarker(t, address); got != "A" {
		t.Fatalf("HTTP marker = %q", got)
	}
	cfg.Listeners[0].Type = config.ListenerSOCKS5

	// When
	prepared, err = manager.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare(protocol swap) error = %v", err)
	}
	manager.Commit(prepared)

	// Then
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		t.Fatalf("Write(greeting) error = %v", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("ReadFull(greeting) error = %v", err)
	}
	if reply[0] != 5 || reply[1] != 0 {
		t.Fatalf("SOCKS5 greeting = %v", reply)
	}
}

func markerProxy(t *testing.T, marker string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close()
				request, readErr := http.ReadRequest(bufio.NewReader(conn))
				if readErr != nil {
					return
				}
				_ = request.Body.Close()
				_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"+marker)
			}()
		}
	}()
	return listener.Addr().String()
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return address
}

func connectMarker(t *testing.T, address string) string {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "CONNECT target.test:443 HTTP/1.1\r\nHost: target.test:443\r\n\r\n"); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}
	marker := make([]byte, 1)
	if _, err := io.ReadFull(response.Body, marker); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	return string(marker)
}
