package fpgonet

import (
	"bufio"
	"io"
	"net"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

// Listen creates a stream listener on the given network and address.
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
		wrapErr(OpNameListen),
	)
}

// Accept accepts the next incoming connection on the listener.
func Accept(l net.Listener) IOE.IOEither[NetError, net.Conn] {
	return IOE.TryCatch(
		l.Accept,
		wrapErr(OpNameAccept),
	)
}

// Dial opens an outbound connection to the given network address.
func Dial(network, addr string) IOE.IOEither[NetError, net.Conn] {
	return IOE.TryCatch(
		func() (net.Conn, error) { return net.Dial(network, addr) },
		wrapErr(OpNameDial),
	)
}

// Read returns a function that reads up to n bytes from a connection.
func Read(n int) func(net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				count, err := c.Read(buf)
				return buf[:count], err
			},
			wrapErr(OpNameRead),
		)
	}
}

// ReadLine reads a newline-terminated line from a connection.
func ReadLine(c net.Conn) IOE.IOEither[NetError, []byte] {
	return ReadLineFrom(bufio.NewReader(c))
}

// ReadLineFrom reads a newline-terminated line from a buffered reader.
func ReadLineFrom(r *bufio.Reader) IOE.IOEither[NetError, []byte] {
	return IOE.TryCatch(
		func() ([]byte, error) { return r.ReadBytes('\n') },
		wrapErr(OpNameReadLine),
	)
}

// Write returns a function that writes b to a connection, returning the byte count written.
func Write(b []byte) func(net.Conn) IOE.IOEither[NetError, int] {
	return func(c net.Conn) IOE.IOEither[NetError, int] {
		return IOE.TryCatch(
			func() (int, error) { return c.Write(b) },
			wrapErr(OpNameWrite),
		)
	}
}

// ReadFull returns a function that reads exactly n bytes from a connection.
func ReadFull(n int) func(net.Conn) IOE.IOEither[NetError, []byte] {
	return func(c net.Conn) IOE.IOEither[NetError, []byte] {
		return IOE.TryCatch(
			func() ([]byte, error) {
				buf := make([]byte, n)
				_, err := io.ReadFull(c, buf)
				return buf, err
			},
			wrapErr(OpNameReadFull),
		)
	}
}

// Close closes a connection or listener.
func Close(c io.Closer) IOE.IOEither[NetError, Void] {
	return IOE.TryCatch(
		func() (struct{}, error) { return VOID, c.Close() },
		wrapErr(OpNameClose),
	)
}

// Handler is a function that processes an accepted connection.
type Handler func(net.Conn) IOE.IOEither[NetError, Void]

// Serve accepts connections from l in a loop, dispatching each to handler in a new goroutine.
// It stops and returns an error if Accept fails.
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
