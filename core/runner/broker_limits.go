package runner

import (
	"net"
	"sync"
)

// Container memory limits must not be bypassed by opening unbounded host-side
// proxy connections. Permits survive HTTP hijacking until the socket closes.
type taskListener struct {
	net.Listener
	permits chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func limitTaskConnections(listener net.Listener, count int) net.Listener {
	return &taskListener{Listener: listener, permits: make(chan struct{}, count), closed: make(chan struct{})}
}

func (l *taskListener) Accept() (net.Conn, error) {
	select {
	case l.permits <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.permits
		return nil, err
	}
	return &taskConnection{Conn: c, release: func() { <-l.permits }}, nil
}

func (l *taskListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

type taskConnection struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *taskConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}
