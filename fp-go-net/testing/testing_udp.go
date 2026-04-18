package testing

import (
	"bytes"
	"net"
	"time"
)

// MockPacketConn implements net.PacketConn for testing UDP packet primitives.
// Writes are captured in an in-memory buffer; reads return the data supplied at construction.
type MockPacketConn struct {
	buf      bytes.Buffer
	// Closed is true after Close has been called.
	Closed   bool
	readData []byte
	readAddr net.Addr
}

var _ net.PacketConn = &MockPacketConn{}

// NewMockPacketConn returns a MockPacketConn pre-loaded with readData and readAddr,
// which will be returned by the first ReadFrom call.
func NewMockPacketConn(readData []byte, readAddr net.Addr) *MockPacketConn {
	return &MockPacketConn{readData: readData, readAddr: readAddr}
}

func (m *MockPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n := copy(b, m.readData)
	return n, m.readAddr, nil
}

func (m *MockPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	return m.buf.Write(b)
}

func (m *MockPacketConn) Close() error                       { m.Closed = true; return nil }
func (m *MockPacketConn) LocalAddr() net.Addr                { return &net.UDPAddr{} }
func (m *MockPacketConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockPacketConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockPacketConn) SetWriteDeadline(t time.Time) error { return nil }

// Written returns everything written to the connection so far.
func (m *MockPacketConn) Written() string { return m.buf.String() }
