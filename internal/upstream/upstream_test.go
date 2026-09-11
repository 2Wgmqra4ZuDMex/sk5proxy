package upstream_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"testing"

	"sk5proxy/internal/config"
	"sk5proxy/internal/upstream"
)

func TestSelector_Activate_routesNewConnectionsWithoutClosingExisting(t *testing.T) {
	// Given
	first := startHTTPProxy(t, "", "")
	second := startHTTPProxy(t, "", "")
	selector, err := upstream.NewSelector([]config.Upstream{
		{ID: "first", Name: "First", Type: config.TypeHTTP, Address: first},
		{ID: "second", Name: "Second", Type: config.TypeHTTP, Address: second},
	}, "first")
	if err != nil {
		t.Fatalf("NewSelector() error = %v", err)
	}
	ctx := context.Background()
	oldConn, err := selector.DialContext(ctx, "tcp", "example.test:443")
	if err != nil {
		t.Fatalf("first DialContext() error = %v", err)
	}
	defer oldConn.Close()

	// When
	if err := selector.Activate("second"); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	newConn, err := selector.DialContext(ctx, "tcp", "example.test:443")
	if err != nil {
		t.Fatalf("second DialContext() error = %v", err)
	}
	defer newConn.Close()

	// Then
	assertEcho(t, oldConn, "old")
	assertEcho(t, newConn, "new")
}

func TestHTTPDialer_DialContext_sendsBasicAuthentication(t *testing.T) {
	// Given
	address := startHTTPProxy(t, "alice", "secret")
	dialer, err := upstream.NewDialer(config.Upstream{ID: "http", Name: "HTTP", Type: config.TypeHTTP, Address: address, Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("NewDialer() error = %v", err)
	}

	// When
	conn, err := dialer.DialContext(context.Background(), "tcp", "target.test:443")

	// Then
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer conn.Close()
	assertEcho(t, conn, "authenticated-http")
}

func TestHTTPDialer_DialContext_preservesBytesBufferedWithConnectResponse(t *testing.T) {
	// Given
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		if _, readErr := http.ReadRequest(bufio.NewReader(conn)); readErr != nil {
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\nimmediate")
	}()
	dialer, err := upstream.NewDialer(config.Upstream{ID: "http", Name: "HTTP", Type: config.TypeHTTP, Address: listener.Addr().String()})
	if err != nil {
		t.Fatalf("NewDialer() error = %v", err)
	}

	// When
	conn, err := dialer.DialContext(context.Background(), "tcp", "target.test:443")
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer conn.Close()
	got := make([]byte, len("immediate"))
	_, err = io.ReadFull(conn, got)

	// Then
	if err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if string(got) != "immediate" {
		t.Fatalf("tunnel bytes = %q", got)
	}
}

func TestSOCKS5Dialer_DialContext_authenticates(t *testing.T) {
	// Given
	address := startSOCKS5Proxy(t, "alice", "secret")
	dialer, err := upstream.NewDialer(config.Upstream{ID: "socks", Name: "SOCKS", Type: config.TypeSOCKS5, Address: address, Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatalf("NewDialer() error = %v", err)
	}

	// When
	conn, err := dialer.DialContext(context.Background(), "tcp", "target.test:443")

	// Then
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer conn.Close()
	assertEcho(t, conn, "authenticated-socks")
}

func startHTTPProxy(t *testing.T, username, password string) string {
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
				req, readErr := http.ReadRequest(bufio.NewReader(conn))
				if readErr != nil {
					return
				}
				want := ""
				if username != "" {
					want = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
				}
				if req.Method != http.MethodConnect || req.Header.Get("Proxy-Authorization") != want {
					_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n")
					return
				}
				_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener.Addr().String()
}

func startSOCKS5Proxy(t *testing.T, username, password string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		header := make([]byte, 2)
		if _, readErr := io.ReadFull(reader, header); readErr != nil {
			return
		}
		methods := make([]byte, int(header[1]))
		_, _ = io.ReadFull(reader, methods)
		_, _ = conn.Write([]byte{5, 2})
		if _, readErr := io.ReadFull(reader, header); readErr != nil {
			return
		}
		user := make([]byte, int(header[1]))
		_, _ = io.ReadFull(reader, user)
		if _, readErr := io.ReadFull(reader, header[:1]); readErr != nil {
			return
		}
		pass := make([]byte, int(header[0]))
		_, _ = io.ReadFull(reader, pass)
		if string(user) != username || string(pass) != password {
			_, _ = conn.Write([]byte{1, 1})
			return
		}
		_, _ = conn.Write([]byte{1, 0})
		request := make([]byte, 4)
		if _, readErr := io.ReadFull(reader, request); readErr != nil {
			return
		}
		if request[3] == 3 {
			if _, readErr := io.ReadFull(reader, header[:1]); readErr != nil {
				return
			}
			_, _ = io.CopyN(io.Discard, reader, int64(header[0])+2)
		}
		_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 1})
		_, _ = io.Copy(conn, conn)
	}()
	return listener.Addr().String()
}

func assertEcho(t *testing.T, conn net.Conn, message string) {
	t.Helper()
	if _, err := conn.Write([]byte(message)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := make([]byte, len(message))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}
	if string(got) != message {
		t.Fatalf("echo = %q, want %q", got, message)
	}
}
