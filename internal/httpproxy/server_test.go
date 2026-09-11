package httpproxy_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"sk5proxy/internal/httpproxy"
)

type directDialer struct{ dialer net.Dialer }

func (d *directDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}

func TestServer_ServeHTTP_forwardsPlainHTTPAndRemovesHopHeaders(t *testing.T) {
	// Given
	received := make(chan http.Header, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		received <- request.Header.Clone()
		w.Header().Set("Connection", "X-Remove")
		w.Header().Set("X-Remove", "secret")
		w.Header().Set("X-End-To-End", "kept")
		_, _ = io.WriteString(w, "forwarded")
	}))
	defer target.Close()
	proxyServer := httptest.NewServer(httpproxy.NewServer(&directDialer{}))
	defer proxyServer.Close()
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Connection", "X-Proxy-Only")
	req.Header.Set("X-Proxy-Only", "remove-me")
	req.Header.Set("Proxy-Authorization", "remove-me")
	req.Header.Set("X-End-To-End", "kept")

	// When
	response, err := client.Do(req)

	// Then
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	headers := <-received
	if headers.Get("X-Proxy-Only") != "" || headers.Get("Proxy-Authorization") != "" {
		t.Fatalf("hop headers reached target: %#v", headers)
	}
	if headers.Get("X-End-To-End") != "kept" || response.Header.Get("X-End-To-End") != "kept" {
		t.Fatal("end-to-end header was removed")
	}
	if response.Header.Get("X-Remove") != "" || string(body) != "forwarded" {
		t.Fatalf("response headers/body = %#v/%q", response.Header, body)
	}
}

func TestServer_ServeHTTP_tunnelsCONNECT(t *testing.T) {
	// Given
	target := startEcho(t)
	proxyServer := httptest.NewServer(httpproxy.NewServer(&directDialer{}))
	defer proxyServer.Close()
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	conn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	// When
	if _, err := io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n"); err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}

	// Then
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	assertEcho(t, conn, "tunneled")
}

func startEcho(t *testing.T) string {
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
		_, _ = io.Copy(conn, conn)
	}()
	return listener.Addr().String()
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
