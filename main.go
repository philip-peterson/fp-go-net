package main

import (
	"fmt"
	"net"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type NetError struct {
	Op  string
	Err error
}

const (
	OpNameAccept   = "accept"
	OpNameListen   = "listen"
	OpNameRead     = "read"
	OpNameWrite    = "write"
	OpNameReadFull = "readFull"
	OpNameClose    = "close"
)

func (e NetError) Error() string { return e.Op + ": " + e.Err.Error() }

type Handler func(net.Conn) IOE.IOEither[NetError, Void]

func Serve(handler Handler) func(net.Listener) IOE.IOEither[NetError, Void] {
	return func(l net.Listener) IOE.IOEither[NetError, Void] {
		return func() E.Either[NetError, Void] {
			for {
				done := false
				E.Fold(
					func(err NetError) Void {
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

var myHandler Handler = func(c net.Conn) IOE.IOEither[NetError, Void] {
	return IOE.Of[NetError](VOID)
}

func main() {

	result := F.Pipe1(
		Listen("tcp", ":8080"),
		IOE.Chain(Serve(myHandler)),
	)()

	E.Fold(
		func(err NetError) Void {
			fmt.Println("fatal:", err)
			return VOID
		},
		func(_ Void) Void { return VOID },
	)(result)
}
