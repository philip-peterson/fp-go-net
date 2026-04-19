// Package fp_go_net_tls provides functional-programming-style TLS wrappers built
// on top of fp-go-net, enabling TLS upgrade of net.Conn connections as
// composable IOEither values.
package fp_go_net_tls

import (
	"crypto/tls"
	"net"

	IOE "github.com/IBM/fp-go/v2/ioeither"
	fp_go_net "github.com/philip-peterson/fp-go-net"
)

func wrapErr(op string) func(error) fp_go_net.NetError {
	return func(err error) fp_go_net.NetError { return fp_go_net.NetError{Op: op, Err: err} }
}

// ServerTLS wraps a plain net.Conn in a server-side TLS session,
// performs the handshake, and returns the upgraded *tls.Conn.
func ServerTLS(cfg *tls.Config) func(net.Conn) IOE.IOEither[fp_go_net.NetError, *tls.Conn] {
	return func(c net.Conn) IOE.IOEither[fp_go_net.NetError, *tls.Conn] {
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
func ClientTLS(cfg *tls.Config) func(net.Conn) IOE.IOEither[fp_go_net.NetError, *tls.Conn] {
	return func(c net.Conn) IOE.IOEither[fp_go_net.NetError, *tls.Conn] {
		return IOE.TryCatch(
			func() (*tls.Conn, error) {
				tlsConn := tls.Client(c, cfg)
				return tlsConn, tlsConn.Handshake()
			},
			wrapErr(OpNameTLSClient),
		)
	}
}
