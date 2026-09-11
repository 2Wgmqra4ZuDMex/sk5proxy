package httpproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Server struct {
	dialer    Dialer
	mu        sync.RWMutex
	transport *http.Transport
}

func NewServer(dialer Dialer) *Server {
	return &Server{dialer: dialer, transport: newTransport(dialer)}
}

func newTransport(dialer Dialer) *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func (s *Server) RoutingChanged() {
	s.mu.Lock()
	previous := s.transport
	s.transport = newTransport(s.dialer)
	s.mu.Unlock()
	previous.CloseIdleConnections()
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	server := &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP proxy: %w", err)
	}
	return nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodConnect {
		s.connect(writer, request)
		return
	}
	s.forward(writer, request)
}

func (s *Server) forward(writer http.ResponseWriter, request *http.Request) {
	outbound := request.Clone(request.Context())
	outbound.RequestURI = ""
	removeHopHeaders(outbound.Header)
	s.mu.RLock()
	transport := s.transport
	s.mu.RUnlock()
	response, err := transport.RoundTrip(outbound)
	if err != nil {
		http.Error(writer, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	removeHopHeaders(response.Header)
	copyHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	_, _ = io.Copy(writer, response.Body)
}

func (s *Server) connect(writer http.ResponseWriter, request *http.Request) {
	remote, err := s.dialer.DialContext(request.Context(), "tcp", request.Host)
	if err != nil {
		http.Error(writer, "Bad Gateway", http.StatusBadGateway)
		return
	}
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		_ = remote.Close()
		http.Error(writer, "Hijacking unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = remote.Close()
		return
	}
	defer client.Close()
	defer remote.Close()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	if buffered.Reader.Buffered() > 0 {
		if _, err := io.CopyN(remote, buffered, int64(buffered.Reader.Buffered())); err != nil {
			return
		}
	}
	pump(request.Context(), client, remote)
}

func removeHopHeaders(header http.Header) {
	for _, connection := range header.Values("Connection") {
		for _, token := range strings.Split(connection, ",") {
			header.Del(strings.TrimSpace(token))
		}
	}
	for _, name := range []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		header.Del(name)
	}
}

func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func pump(ctx context.Context, left, right net.Conn) {
	stop := context.AfterFunc(ctx, func() {
		_ = left.Close()
		_ = right.Close()
	})
	defer stop()
	var wait sync.WaitGroup
	wait.Add(2)
	copyOne := func(destination, source net.Conn) {
		defer wait.Done()
		_, _ = io.Copy(destination, source)
		if closer, ok := destination.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
	}
	go copyOne(left, right)
	go copyOne(right, left)
	wait.Wait()
}
