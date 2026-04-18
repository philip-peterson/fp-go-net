package main

import (
	"fmt"
	"net"

	. "github.com/IBM/fp-go/v2/function"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

var myHandler fpnet.Handler = func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 11\r\n\r\nHello World")
	return Pipe1(
		fpnet.Write(response)(c),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return fpnet.Close(c)
		}),
	)
}

func main() {
	port := ":8080"

	Pipe3(
		fpnet.Listen("tcp", port),
		IOE.ChainFirst(func(l net.Listener) IOE.IOEither[fpnet.NetError, net.Listener] {
			return IOE.FromIO[fpnet.NetError](func() net.Listener {
				fmt.Println("listening on", port)
				return l
			})
		}),
		IOE.Chain(fpnet.Serve(myHandler)),
		IOE.Fold(
			func(err fpnet.NetError) IO.IO[Void] {
				return func() Void { fmt.Println("fatal:", err); return VOID }
			},
			func(_ Void) IO.IO[Void] {
				return func() Void { return VOID }
			},
		),
	)()
}
