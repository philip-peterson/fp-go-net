package main

import (
	"fmt"
	"net"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

var myHandler fpnet.Handler = func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 11\r\n\r\nHello World")
	return F.Pipe1(
		fpnet.Write(response)(c),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return fpnet.Close(c)
		}),
	)
}

func main() {

	result := F.Pipe1(
		fpnet.Listen("tcp", ":8080"),
		IOE.Chain(fpnet.Serve(myHandler)),
	)()

	E.Fold(
		func(err fpnet.NetError) Void {
			fmt.Println("fatal:", err)
			return VOID
		},
		func(_ Void) Void { return VOID },
	)(result)
}
