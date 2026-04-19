package fp_go_net

import (
	"net"

	IOE "github.com/IBM/fp-go/v2/ioeither"
)

// Packet holds a received datagram and its source address.
type Packet struct {
	// Data contains the raw bytes of the received datagram.
	Data []byte
	// Addr is the source address of the datagram.
	Addr net.Addr
}

// ListenPacket creates a packet-oriented listener on the given network and address.
func ListenPacket(network, addr string) IOE.IOEither[NetError, net.PacketConn] {
	return IOE.TryCatch(
		func() (net.PacketConn, error) { return net.ListenPacket(network, addr) },
		wrapErr(OpNameListenPacket),
	)
}

// ReadFrom returns a function that reads one datagram of up to n bytes from a PacketConn.
func ReadFrom(n int) func(net.PacketConn) IOE.IOEither[NetError, Packet] {
	return func(c net.PacketConn) IOE.IOEither[NetError, Packet] {
		return IOE.TryCatch(
			func() (Packet, error) {
				buf := make([]byte, n)
				n, addr, err := c.ReadFrom(buf)
				return Packet{buf[:n], addr}, err
			},
			func(err error) NetError { return NetError{Op: OpNameReadFrom, Err: err} },
		)
	}
}

// WriteTo returns a function that writes b to addr on a PacketConn, returning the byte count written.
func WriteTo(b []byte, addr net.Addr) func(net.PacketConn) IOE.IOEither[NetError, int] {
	return func(c net.PacketConn) IOE.IOEither[NetError, int] {
		return IOE.TryCatch(
			func() (int, error) { return c.WriteTo(b, addr) },
			func(err error) NetError { return NetError{Op: OpNameWriteTo, Err: err} },
		)
	}
}
