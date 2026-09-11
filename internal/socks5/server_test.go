package socks5_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"sk5proxy/internal/socks5"
)

type recordingDialer struct{ addresses chan string }

func (d *recordingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.addresses <- address
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		_, _ = io.Copy(server, server)
	}()
	return client, nil
}

func TestServer_CONNECT_supportsIPv4IPv6AndDomain(t *testing.T) {
	tests := []struct {
		name    string
		address string
		request []byte
	}{
		{name: "ipv4", address: "127.0.0.1:8080", request: append([]byte{5, 1, 0, 1, 127, 0, 0, 1}, 0x1f, 0x90)},
		{name: "ipv6", address: "[::1]:8080", request: append(append([]byte{5, 1, 0, 4}, net.ParseIP("::1").To16()...), 0x1f, 0x90)},
		{name: "domain", address: "example.test:8080", request: append(append([]byte{5, 1, 0, 3, 12}, []byte("example.test")...), 0x1f, 0x90)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dialer := &recordingDialer{addresses: make(chan string, 1)}
			client, server := net.Pipe()
			done := make(chan error, 1)
			go func() { done <- socks5.NewServer(dialer).Handle(context.Background(), server) }()
			defer client.Close()

			// When
			if _, err := client.Write([]byte{5, 1, 0}); err != nil {
				t.Fatalf("write greeting: %v", err)
			}
			method := make([]byte, 2)
			if _, err := io.ReadFull(client, method); err != nil {
				t.Fatalf("read greeting: %v", err)
			}
			if _, err := client.Write(tt.request); err != nil {
				t.Fatalf("write request: %v", err)
			}
			reply := make([]byte, 10)
			if _, err := io.ReadFull(client, reply); err != nil {
				t.Fatalf("read reply: %v", err)
			}

			// Then
			if method[1] != 0 || reply[1] != 0 {
				t.Fatalf("method/reply = %v/%v", method, reply)
			}
			if got := <-dialer.addresses; got != tt.address {
				t.Fatalf("dialed %q, want %q", got, tt.address)
			}
			assertEcho(t, client, "payload")
		})
	}
}

func TestServer_Handle_rejectsUnsupportedCommand(t *testing.T) {
	// Given
	dialer := &recordingDialer{addresses: make(chan string, 1)}
	client, server := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- socks5.NewServer(dialer).Handle(context.Background(), server) }()
	defer client.Close()
	_, _ = client.Write([]byte{5, 1, 0})
	_, _ = io.ReadFull(client, make([]byte, 2))

	// When
	request := []byte{5, 2, 0, 1, 127, 0, 0, 1, 0, 80}
	_, _ = client.Write(request)
	reply := make([]byte, 10)
	_, err := io.ReadFull(client, reply)

	// Then
	if err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if reply[1] != 7 {
		t.Fatalf("reply code = %d, want 7", reply[1])
	}
	if handleErr := <-done; handleErr == nil {
		t.Fatal("Handle() error = nil")
	}
}

func TestServer_Handle_stopsTunnelWhenContextCancelled(t *testing.T) {
	// Given
	dialer := &recordingDialer{addresses: make(chan string, 1)}
	client, server := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- socks5.NewServer(dialer).Handle(ctx, server) }()
	defer client.Close()
	_, _ = client.Write([]byte{5, 1, 0})
	_, _ = io.ReadFull(client, make([]byte, 2))
	_, _ = client.Write([]byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 80})
	_, _ = io.ReadFull(client, make([]byte, 10))

	// When
	cancel()

	// Then
	waitCtx, stopWaiting := context.WithTimeout(context.Background(), time.Second)
	defer stopWaiting()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
	case <-waitCtx.Done():
		t.Fatal("Handle() did not stop after cancellation")
	}
}

func assertEcho(t *testing.T, conn net.Conn, value string) {
	t.Helper()
	if _, err := conn.Write([]byte(value)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := make([]byte, len(value))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if string(got) != value {
		t.Fatalf("echo = %q, want %q", got, value)
	}
}
