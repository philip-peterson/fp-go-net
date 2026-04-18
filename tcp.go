package fpgonet

import (
	"bufio"
	"io"
	"net"

	E "github.com/IBM/fp-go/v2/either"
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
		func(err error) NetError { return NetError{"listen", err} },
	)
}

func Accept(l net.Listener) IOE.IOEither[NetError, net.Conn] {
	return IOE.TryCatch(
		l.Accept,
		func(err error) NetError { return NetError{"accept", err} },
	)
}

func Read(n int) func(net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				count, err := c.Read(buf)
				return buf[:count], err
			},
			func(err error) NetError { return NetError{"read", err} },
		)
	}
}

func ReadLine(c net.Conn) IOE.IOEither[NetError, []byte] {
	return ReadLineFrom(bufio.NewReader(c))
}

func ReadLineFrom(r *bufio.Reader) IOE.IOEither[NetError, []byte] {
	return IOE.TryCatch(
		func() ([]byte, error) { return r.ReadBytes('\n') },
		func(err error) NetError { return NetError{"readLine", err} },
	)
}

func Write(b []byte) func(net.Conn) IOE.IOEither[NetError, int] {
	return func(c net.Conn) IOE.IOEither[NetError, int] {
		return IOE.TryCatch(
			func() (int, error) { return c.Write(b) },
			func(err error) NetError { return NetError{"write", err} },
		)
	}
}

func ReadFull(n int) func(net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				_, err := io.ReadFull(c, buf)
				return buf, err
			},
			func(err error) NetError { return NetError{"readFull", err} },
		)
	}
}

func Close(c io.Closer) IOE.IOEither[NetError, Void] {
	return IOE.TryCatch(
		func() (struct{}, error) { return VOID, c.Close() },
		func(err error) NetError { return NetError{"close", err} },
	)
}

type Handler func(net.Conn) IOE.IOEither[NetError, Void]

func Serve(handler Handler) func(net.Listener) IOE.IOEither[NetError, Void] {
	return func(l net.Listener) IOE.IOEither[NetError, Void] {
		return func() E.Either[NetError, Void] {
			for {
				done := false
				E.Fold(
					func(NetError) Void {
						done = true
						return VOID
					},
					func(conn net.Conn) Void {
						go handler(conn)()
						return VOID
					},
				)(Accept(l)())
				if done {
					break
				}
			}
			return E.Right[NetError](VOID)
		}
	}
}
