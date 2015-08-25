package fakenet

import (
	"fmt"
	"net"
	"sync"

	"golang.org/x/net/context"
)

// Network creates a network that will track listeners it created and
// provides a Dial func to connect with them.
func Network() *FakeNetwork {
	return &FakeNetwork{listeners: make(map[string]*listener)}
}

// FakeNetwork tracks listeners it created and
// provides a Dial func to connect with them.
type FakeNetwork struct {
	mu        sync.Mutex
	listeners map[string]*listener
}

// Dial a listener at the given address. The address should come from
// a listener that was created by this network.
func (net *FakeNetwork) Dial(ctx context.Context, addr string) (net.Conn, error) {
	net.mu.Lock()
	l, ok := net.listeners[addr]
	net.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("fakenet: host %q doesn't exist", addr)
	}
	select {
	case conn := <-l.acceptc:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Listen creates a listener on the network.
func (net *FakeNetwork) Listen(parent context.Context) net.Listener {
	ctx, cancel := context.WithCancel(parent)
	l := &listener{
		ctx:     ctx,
		addr:    newAddr(),
		acceptc: make(chan *conn, 0),
		conns:   make(map[*conn]struct{}),
	}
	net.mu.Lock()
	net.listeners[l.addr.String()] = l
	net.mu.Unlock()
	l.cancel = func() {
		net.mu.Lock()
		delete(net.listeners, l.addr.String())
		net.mu.Unlock()
		cancel()
	}
	return l
}
