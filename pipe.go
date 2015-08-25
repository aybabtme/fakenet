package fakenet

import (
	"io"
	"sync"

	"golang.org/x/net/context"
)

func newDuplex(ctx context.Context) (*pipe, *pipe) {
	forward := make(chan []byte, 0)
	backward := make(chan []byte, 0)
	fwpipe := &pipe{
		bgCtx: ctx,
		w:     &pipeWriter{bgCtx: ctx, writech: forward},
		r:     &pipeReader{bgCtx: ctx, readch: backward},
	}
	bwpipe := &pipe{
		bgCtx: ctx,
		w:     &pipeWriter{bgCtx: ctx, writech: backward},
		r:     &pipeReader{bgCtx: ctx, readch: forward},
	}
	return fwpipe, bwpipe
}

type pipe struct {
	bgCtx context.Context
	r     *pipeReader
	w     *pipeWriter
}

func (pipe *pipe) Write(p []byte) (int, error)                         { return pipe.w.Write(pipe.bgCtx, p) }
func (pipe *pipe) Read(p []byte) (int, error)                          { return pipe.r.Read(pipe.bgCtx, p) }
func (pipe *pipe) CtxWrite(ctx context.Context, p []byte) (int, error) { return pipe.w.Write(ctx, p) }
func (pipe *pipe) CtxRead(ctx context.Context, p []byte) (int, error)  { return pipe.r.Read(ctx, p) }
func (pipe *pipe) Close() error                                        { return pipe.w.Close() }

type pipeWriter struct {
	mu        sync.Mutex
	bgCtx     context.Context
	closeOnce sync.Once
	writech   chan<- []byte
}

func (pw *pipeWriter) Write(ctx context.Context, p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	payload := make([]byte, len(p))
	copy(payload, p)
	select {
	case <-pw.bgCtx.Done():
		return 0, pw.bgCtx.Err()
	case <-ctx.Done():
		return 0, ctx.Err()
	case pw.writech <- payload:
	}
	return len(p), nil
}

func (pw *pipeWriter) Close() error {
	pw.closeOnce.Do(func() {
		close(pw.writech)
	})
	return nil
}

type pipeReader struct {
	mu     sync.Mutex
	bgCtx  context.Context
	buf    []byte
	wasEOF bool
	readch <-chan []byte
}

func (pr *pipeReader) Read(ctx context.Context, p []byte) (int, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	select {
	case <-pr.bgCtx.Done():
		return 0, pr.bgCtx.Err()
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// read from the buffer first
	if len(pr.buf) != 0 {
		if len(p) > len(pr.buf) {
			copy(p, pr.buf[:len(p)])
			pr.buf = pr.buf[len(pr.buf)-len(p):]
			return len(p), nil

		}
		n := len(pr.buf)
		copy(p, pr.buf)
		pr.buf = nil
		if pr.wasEOF {

			return n, io.EOF
		}
		return n, nil
	}

	// otherwise get data from channel
	select {
	case <-pr.bgCtx.Done():
		return 0, pr.bgCtx.Err()
	case <-ctx.Done():
		return 0, ctx.Err()
	case payload, more := <-pr.readch:
		if len(payload) > len(p) {
			pr.buf = payload[len(payload)-len(p):]
			payload = payload[:len(p)]

		}

		copy(p, payload)

		if !more && pr.buf == nil {

			return len(payload), io.EOF
		} else if !more {
			pr.wasEOF = true
		}

		return len(payload), nil
	}
}
