package main

import (
	"log"
	"net"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

func serveUDP(c net.PacketConn) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		for {
			done := false
			E.Fold(
				func(err fpnet.NetError) Void {
					log.Println("read error:", err)
					done = true
					return VOID
				},
				func(packet fpnet.Packet) Void {
					log.Printf("got %q from %s", packet.Data, packet.Addr)
					E.Fold(
						func(err fpnet.NetError) Void {
							log.Println("write error:", err)
							done = true
							return VOID
						},
						func(_ int) Void { return VOID },
					)(fpnet.WriteTo(packet.Data, packet.Addr)(c)())
					return VOID
				},
			)(fpnet.ReadFrom(1024)(c)())
			if done {
				break
			}
		}
		return E.Right[fpnet.NetError](VOID)
	}
}

func main() {
	port := ":9090"
	Pipe3(
		fpnet.ListenPacket("udp", port),
		IOE.ChainFirst(func(c net.PacketConn) IOE.IOEither[fpnet.NetError, Void] {
			return IOE.FromIO[fpnet.NetError](func() Void {
				log.Println("UDP listening on", port)
				return VOID
			})
		}),
		IOE.Chain(serveUDP),
		IOE.Fold(
			func(err fpnet.NetError) IO.IO[Void] {
				return func() Void { log.Fatal("fatal:", err); return VOID }
			},
			func(_ Void) IO.IO[Void] {
				return func() Void { return VOID }
			},
		),
	)()
}
