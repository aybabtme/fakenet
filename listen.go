package fakenet

import (
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"
)

var (
	ErrClosing = errors.New("fakenet: listener is closing")
)

var _ net.Addr = new(addr)

type addr string

func newAddr() net.Addr       { return addr(randString(64) + ":80") }
func (addr) Network() string  { return "fakenetwork" }
func (a addr) String() string { return string(a) }

var _ net.Listener = new(listener)

type listener struct {
	ctx    context.Context
	cancel func()

	addr    net.Addr
	acceptc chan *conn

	connsl sync.Mutex
	conns  map[*conn]struct{}
}

func (l *listener) Addr() net.Addr { return l.addr }
func (l *listener) Accept() (net.Conn, error) {
	select {
	case <-l.ctx.Done():
		return nil, ErrClosing
	default:
	}
	client, server := newConn(l.ctx, l.addr, func(client, server *conn) {
		l.connsl.Lock()
		delete(l.conns, client)
		delete(l.conns, server)
		l.connsl.Unlock()
	})
	l.connsl.Lock()
	l.conns[client] = struct{}{}
	l.conns[server] = struct{}{}
	l.connsl.Unlock()
	select {
	case l.acceptc <- client:
		return server, nil
	case <-l.ctx.Done():
		return nil, ErrClosing
	}
}

func (l listener) Close() error {
	l.cancel()
	l.connsl.Lock()
	defer l.connsl.Unlock()
	for conn := range l.conns {
		conn.Close()
	}
	l.conns = nil
	return nil
}

var _ net.Conn = new(conn)

type conn struct {
	laddr, raddr net.Addr

	bgCtx context.Context

	deadlinel     sync.Mutex
	readdeadline  time.Time
	writedeadline time.Time

	pipe *pipe

	close func() error
}

func newConn(ctx context.Context, serverAddr net.Addr, onClose func(client, server *conn)) (*conn, *conn) {
	clientAddr := newAddr()

	fw, bw := newDuplex(ctx)

	client := &conn{
		laddr: clientAddr, raddr: serverAddr,
		bgCtx: ctx,
		pipe:  fw,
	}
	server := &conn{
		laddr: serverAddr, raddr: clientAddr,
		bgCtx: ctx,
		pipe:  bw,
	}
	client.close = func() error { onClose(client, server); return nil }
	server.close = func() error { onClose(client, server); return nil }

	go func() {
		<-ctx.Done()
		onClose(client, server)
	}()

	return client, server
}

func (c *conn) LocalAddr() net.Addr  { return c.laddr }
func (c *conn) RemoteAddr() net.Addr { return c.raddr }

func (c *conn) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	err2 := c.SetWriteDeadline(t)
	if err == nil {
		err = err2
	}
	return err
}

func (c *conn) SetReadDeadline(t time.Time) error {
	c.deadlinel.Lock()
	defer c.deadlinel.Unlock()
	c.readdeadline = t
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	c.deadlinel.Lock()
	defer c.deadlinel.Unlock()
	c.writedeadline = t
	return nil
}

func (c *conn) Read(p []byte) (int, error) {
	c.deadlinel.Lock()
	deadline := c.readdeadline
	c.deadlinel.Unlock()

	var (
		n   int
		err error
	)
	if !deadline.IsZero() {
		ctx, cancel := context.WithDeadline(c.bgCtx, deadline)
		defer cancel()
		n, err = c.pipe.CtxRead(ctx, p)
	} else {
		n, err = c.pipe.Read(p)
	}
	return n, err
}

func (c *conn) Write(p []byte) (int, error) {
	c.deadlinel.Lock()
	deadline := c.writedeadline
	c.deadlinel.Unlock()

	if !deadline.IsZero() {
		ctx, cancel := context.WithDeadline(c.bgCtx, deadline)
		defer cancel()
		return c.pipe.CtxWrite(ctx, p)
	}
	return c.pipe.Write(p)
}

func (c *conn) Close() error {
	c.close()
	return c.pipe.Close()
}

func randString(l int) string {
	const abc = "abcdefghijklmnopqrstuvwxyz"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	buf := make([]byte, l)
	for i := range buf {
		buf[i] = abc[r.Intn(len(abc))]
	}
	return string(buf)
}
