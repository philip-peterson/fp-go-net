// Package testing provides test helpers for fp-go-net.
package testing

import (
	"bytes"
	"net"
	"testing"
	"time"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"

	fpnet "github.com/philip-peterson/fp-go-net"
)

// AssertRight fails the test if result is a Left, and returns the Right value.
func AssertRight[A any](t *testing.T, result E.Either[fpnet.NetError, A]) A {
	t.Helper()
	var val A
	E.Fold(
		func(err fpnet.NetError) Void {
			t.Fatalf("expected Right, got Left: %v", err)
			return VOID
		},
		func(a A) Void {
			val = a
			return VOID
		},
	)(result)
	return val
}

// MockConn implements net.Conn, capturing writes in an in-memory buffer.
// Reads always return immediately with no data. Useful for testing handlers
// without any real network connections.
type MockConn struct {
	buf    bytes.Buffer
	Closed bool
}

func (m *MockConn) Write(b []byte) (int, error)        { return m.buf.Write(b) }
func (m *MockConn) Read(b []byte) (int, error)         { return 0, nil }
func (m *MockConn) Close() error                       { m.Closed = true; return nil }
func (m *MockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *MockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *MockConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConn) SetWriteDeadline(t time.Time) error { return nil }

// Written returns everything written to the connection so far.
func (m *MockConn) Written() string { return m.buf.String() }
