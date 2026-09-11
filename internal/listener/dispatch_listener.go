package listener

import (
	"context"
	"net"
	"sync"
)

type dispatchListener struct {
	address net.Addr
	conns   chan net.Conn
	done    chan struct{}
	once    sync.Once
}

func newDispatchListener(address net.Addr) *dispatchListener {
	return &dispatchListener{
		address: address,
		conns:   make(chan net.Conn),
		done:    make(chan struct{}),
	}
}

func (l *dispatchListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *dispatchListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *dispatchListener) Addr() net.Addr { return l.address }

func (l *dispatchListener) deliver(ctx context.Context, conn net.Conn) error {
	select {
	case l.conns <- conn:
		return nil
	case <-l.done:
		return net.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}
