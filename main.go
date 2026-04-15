package fpgonet

import (
	"net"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type NetError struct {
	Op  string
	Err error
}

func (e NetError) Error() string { return e.Op + ": " + e.Err.Error() }

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
