package fpgonet

import (
	"net"

	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type Packet struct {
	Data []byte
	Addr net.Addr
}

func ListenPacket(network, addr string) IOE.IOEither[NetError, net.PacketConn] {
	return IOE.TryCatch(
		func() (net.PacketConn, error) { return net.ListenPacket(network, addr) },
		wrapErr(OpNameListenPacket),
	)
}

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

func WriteTo(b []byte, addr net.Addr) func(net.PacketConn) IOE.IOEither[NetError, int] {
	return func(c net.PacketConn) IOE.IOEither[NetError, int] {
		return IOE.TryCatch(
			func() (int, error) { return c.WriteTo(b, addr) },
			func(err error) NetError { return NetError{Op: OpNameWriteTo, Err: err} },
		)
	}
}
