// Package fpgonettls provides functional-programming-style TLS wrappers built
// on top of fp-go-net, enabling TLS upgrade of net.Conn connections as
// composable IOEither values.
package fpgonettls

import (
	"crypto/tls"
	"net"

	IOE "github.com/IBM/fp-go/v2/ioeither"
	fpgonet "github.com/philip-peterson/fp-go-net"
)

func wrapErr(op string) func(error) fpgonet.NetError {
	return func(err error) fpgonet.NetError { return fpgonet.NetError{Op: op, Err: err} }
}

// ServerTLS wraps a plain net.Conn in a server-side TLS session,
// performs the handshake, and returns the upgraded *tls.Conn.
func ServerTLS(cfg *tls.Config) func(net.Conn) IOE.IOEither[fpgonet.NetError, *tls.Conn] {
	return func(c net.Conn) IOE.IOEither[fpgonet.NetError, *tls.Conn] {
		return IOE.TryCatch(
			func() (*tls.Conn, error) {
				tlsConn := tls.Server(c, cfg)
				return tlsConn, tlsConn.Handshake()
			},
			wrapErr(OpNameTLSServer),
		)
	}
}

// ClientTLS wraps a plain net.Conn in a client-side TLS session,
// performs the handshake, and returns the upgraded *tls.Conn.
func ClientTLS(cfg *tls.Config) func(net.Conn) IOE.IOEither[fpgonet.NetError, *tls.Conn] {
	return func(c net.Conn) IOE.IOEither[fpgonet.NetError, *tls.Conn] {
		return IOE.TryCatch(
			func() (*tls.Conn, error) {
				tlsConn := tls.Client(c, cfg)
				return tlsConn, tlsConn.Handshake()
			},
			wrapErr(OpNameTLSClient),
		)
	}
}
