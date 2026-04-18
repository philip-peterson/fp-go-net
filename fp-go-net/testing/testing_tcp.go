// Package testing provides test helpers for fp-go-net.
package testing

import (
	"bytes"
	"net"
	"time"
)

// MockConn implements net.Conn for testing TCP stream primitives.
// Writes are captured in an in-memory buffer; reads return immediately with no data.
type MockConn struct {
	buf    bytes.Buffer
	// Closed is true after Close has been called.
	Closed bool
}

var _ net.Conn = &MockConn{}

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
