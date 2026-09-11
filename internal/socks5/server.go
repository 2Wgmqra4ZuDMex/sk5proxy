package socks5

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
)

var (
	ErrProtocol           = errors.New("invalid SOCKS5 protocol")
	ErrUnsupportedCommand = errors.New("unsupported SOCKS5 command")
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Server struct{ dialer Dialer }

func NewServer(dialer Dialer) *Server { return &Server{dialer: dialer} }

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept SOCKS5 connection: %w", err)
		}
		go func() { _ = s.Handle(ctx, conn) }()
	}
}

func (s *Server) Handle(ctx context.Context, client net.Conn) error {
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	if err := negotiate(client); err != nil {
		return err
	}
	command, address, err := readRequest(client)
	if err != nil {
		_ = writeReply(client, 1, nil)
		return err
	}
	if command != 1 {
		if writeErr := writeReply(client, 7, nil); writeErr != nil {
			return errors.Join(ErrUnsupportedCommand, writeErr)
		}
		return ErrUnsupportedCommand
	}
	remote, err := s.dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		_ = writeReply(client, 5, nil)
		return fmt.Errorf("dial target %s: %w", address, err)
	}
	defer remote.Close()
	if err := writeReply(client, 0, remote.LocalAddr()); err != nil {
		return err
	}
	return pump(ctx, client, remote)
}

func negotiate(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read greeting: %w", err)
	}
	if header[0] != 5 {
		return ErrProtocol
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read methods: %w", err)
	}
	for _, method := range methods {
		if method == 0 {
			if _, err := conn.Write([]byte{5, 0}); err != nil {
				return fmt.Errorf("write method: %w", err)
			}
			return nil
		}
	}
	_, err := conn.Write([]byte{5, 0xff})
	if err != nil {
		return fmt.Errorf("reject methods: %w", err)
	}
	return ErrProtocol
}

func readRequest(conn net.Conn) (byte, string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, "", fmt.Errorf("read request: %w", err)
	}
	if header[0] != 5 || header[2] != 0 {
		return 0, "", ErrProtocol
	}
	host, err := readHost(conn, header[3])
	if err != nil {
		return 0, "", err
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		return 0, "", fmt.Errorf("read port: %w", err)
	}
	return header[1], net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), nil
}

func readHost(conn net.Conn, addressType byte) (string, error) {
	switch addressType {
	case 1:
		address := make([]byte, net.IPv4len)
		_, err := io.ReadFull(conn, address)
		return net.IP(address).String(), err
	case 4:
		address := make([]byte, net.IPv6len)
		_, err := io.ReadFull(conn, address)
		return net.IP(address).String(), err
	case 3:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", fmt.Errorf("read domain length: %w", err)
		}
		address := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", fmt.Errorf("read domain: %w", err)
		}
		return string(address), nil
	default:
		return "", ErrProtocol
	}
}

func writeReply(conn net.Conn, code byte, address net.Addr) error {
	ip := net.IPv4zero
	port := 0
	if tcpAddress, ok := address.(*net.TCPAddr); ok {
		ip = tcpAddress.IP.To4()
		port = tcpAddress.Port
		if ip == nil {
			ip = net.IPv4zero
		}
	}
	reply := []byte{5, code, 0, 1, ip[0], ip[1], ip[2], ip[3], 0, 0}
	binary.BigEndian.PutUint16(reply[8:], uint16(port))
	if _, err := conn.Write(reply); err != nil {
		return fmt.Errorf("write reply: %w", err)
	}
	return nil
}

func pump(ctx context.Context, left, right net.Conn) error {
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
	return nil
}
