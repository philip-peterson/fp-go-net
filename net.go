package main

import (
	"io"
	"net"

	. "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

func Listen(network string, addr string) IOE.IOEither[NetError, net.Listener] {
	return listenInternal(net.Listen, network, addr)
}

func listenInternal(
	listenFunc func(network, addr string) (net.Listener, error),
	network string,
	addr string,
) IOE.IOEither[NetError, net.Listener] {
	return IOE.TryCatch(
		func() (net.Listener, error) { return listenFunc(network, addr) },
		func(err error) NetError { return NetError{OpNameListen, err} },
	)
}

func Accept(l net.Listener) IOE.IOEither[NetError, net.Conn] {
	return IOE.TryCatch(
		l.Accept,
		func(err error) NetError { return NetError{OpNameAccept, err} },
	)
}

func Read(n int) func(c net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				count, err := c.Read(buf)
				return buf[:count], err
			},
			func(err error) NetError { return NetError{OpNameRead, err} },
		)
	}
}

func Write(b []byte) func(net.Conn) IOE.IOEither[NetError, int] {
	return func(c net.Conn) IOE.IOEither[NetError, int] {
		return IOE.TryCatch(
			func() (int, error) { return c.Write(b) },
			func(err error) NetError { return NetError{OpNameWrite, err} },
		)
	}
}

func ReadFull(n int) func(c net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				_, err := io.ReadFull(c, buf)
				return buf, err
			},
			func(err error) NetError { return NetError{OpNameReadFull, err} },
		)
	}
}

func Close(c io.Closer) IOE.IOEither[NetError, Void] {
	return IOE.TryCatch(
		func() (struct{}, error) { return VOID, c.Close() },
		func(err error) NetError { return NetError{OpNameClose, err} },
	)
}
