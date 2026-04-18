package testing

import (
	"bytes"
	"net"
	"time"
)

// MockPacketConn implements net.PacketConn for testing UDP primitives.
// Compare to MockConn.
type MockPacketConn struct {
	buf      bytes.Buffer
	Closed   bool
	readData []byte
	readAddr net.Addr
}

var _ net.PacketConn = &MockPacketConn{}

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

func (m *MockPacketConn) Written() string { return m.buf.String() }
