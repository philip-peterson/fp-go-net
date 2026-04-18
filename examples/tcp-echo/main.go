package main

import (
	"fmt"
	"net"

	. "github.com/IBM/fp-go/v2/function"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

var handler fpnet.Handler = func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return Pipe2(
		fpnet.Read(1024)(c),
		IOE.Chain(func(b []byte) IOE.IOEither[fpnet.NetError, int] {
			return fpnet.Write(b)(c)
		}),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return fpnet.Close(c)
		}),
	)
}

func main() {
	Pipe2(
		fpnet.Listen("tcp", ":9000"),
		IOE.Chain(fpnet.Serve(handler)),
		IOE.Fold(
			func(err fpnet.NetError) IO.IO[Void] {
				return func() Void { fmt.Println("error:", err); return VOID }
			},
			func(_ Void) IO.IO[Void] {
				return func() Void { return VOID }
			},
		),
	)()
}
