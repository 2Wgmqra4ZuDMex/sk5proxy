package upstream

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"

	"sk5proxy/internal/config"
)

var (
	ErrUnsupportedType = errors.New("unsupported upstream type")
	ErrProxyResponse   = errors.New("upstream proxy rejected connection")
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func NewDialer(item config.Upstream) (Dialer, error) {
	netDialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	switch item.Type {
	case config.TypeSOCKS5:
		var auth *proxy.Auth
		if item.Username != "" || item.Password != "" {
			auth = &proxy.Auth{User: item.Username, Password: item.Password}
		}
		dialer, err := proxy.SOCKS5("tcp", item.Address, auth, netDialer)
		if err != nil {
			return nil, fmt.Errorf("create SOCKS5 dialer: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("SOCKS5 context support: %w", ErrUnsupportedType)
		}
		return contextDialerAdapter{dialer: contextDialer}, nil
	case config.TypeHTTP:
		return &httpConnectDialer{address: item.Address, username: item.Username, password: item.Password, dialer: netDialer}, nil
	default:
		return nil, fmt.Errorf("upstream type %q: %w", item.Type, ErrUnsupportedType)
	}
}

type contextDialerAdapter struct{ dialer proxy.ContextDialer }

func (d contextDialerAdapter) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("dial SOCKS5 upstream: %w", err)
	}
	return conn, nil
}

type httpConnectDialer struct {
	address  string
	username string
	password string
	dialer   *net.Dialer
}

func (d *httpConnectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dialer.DialContext(ctx, network, d.address)
	if err != nil {
		return nil, fmt.Errorf("dial HTTP upstream %s: %w", d.address, err)
	}
	success := false
	defer func() {
		if !success {
			_ = conn.Close()
		}
	}()
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Host: address}, Host: address, Header: make(http.Header)}
	if d.username != "" || d.password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(d.username + ":" + d.password))
		req.Header.Set("Proxy-Authorization", "Basic "+token)
	}
	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("write HTTP CONNECT: %w", err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, fmt.Errorf("read HTTP CONNECT response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		if closeErr := response.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("close HTTP CONNECT response: %w", closeErr)
		}
		return nil, &ResponseError{StatusCode: response.StatusCode}
	}
	success = true
	return &bufferedConn{Conn: conn, reader: reader}, nil
}

type bufferedConn struct {
	net.Conn
	reader io.Reader
}

func (c *bufferedConn) Read(buffer []byte) (int, error) { return c.reader.Read(buffer) }

type ResponseError struct{ StatusCode int }

func (e *ResponseError) Error() string        { return fmt.Sprintf("upstream proxy status %d", e.StatusCode) }
func (e *ResponseError) Is(target error) bool { return target == ErrProxyResponse }
