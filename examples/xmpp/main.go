package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"
	"github.com/IBM/fp-go/v2/ioref"
	"github.com/IBM/fp-go/v2/pair"

	fpnet "github.com/philip-peterson/fp-go-net"
	fpnettls "github.com/philip-peterson/fp-go-net/fp-go-net-tls"
)

const domain = "localhost"

type JID = string

// session holds the active connection and its XML decoder. After STARTTLS the
// raw net.Conn is replaced by a *tls.Conn; everything downstream uses session
// so the upgrade is invisible to saslPhase, bindPhase, and stanzaLoop.
type session struct {
	conn net.Conn
	dec  *xml.Decoder
}

type ServerState struct {
	sessions map[JID]net.Conn
}

func newState() ServerState {
	return ServerState{sessions: make(map[JID]net.Conn)}
}

// XML stanza types

type Auth struct {
	XMLName   xml.Name `xml:"auth"`
	Mechanism string   `xml:"mechanism,attr"`
	Value     string   `xml:",chardata"`
}

type BindIQ struct {
	XMLName  xml.Name `xml:"iq"`
	Type     string   `xml:"type,attr"`
	ID       string   `xml:"id,attr"`
	Resource string   `xml:"bind>resource"`
}

type Message struct {
	XMLName xml.Name `xml:"message"`
	To      string   `xml:"to,attr"`
	From    string   `xml:"from,attr,omitempty"`
	Body    string   `xml:"body"`
}

// Low-level helpers

func wrapXMPP(op string) func(error) fpnet.NetError {
	return func(err error) fpnet.NetError { return fpnet.NetError{Op: op, Err: err} }
}

func writeVoid(c net.Conn, s string) IOE.IOEither[fpnet.NetError, Void] {
	return Pipe1(
		fpnet.Write([]byte(s))(c),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func readStartEl(dec *xml.Decoder) IOE.IOEither[fpnet.NetError, xml.StartElement] {
	return IOE.TryCatch(
		func() (xml.StartElement, error) {
			for {
				tok, err := dec.Token()
				if err != nil {
					return xml.StartElement{}, err
				}
				if se, ok := tok.(xml.StartElement); ok {
					return se, nil
				}
			}
		},
		wrapXMPP("read"),
	)
}

func decodeEl[T any](dec *xml.Decoder, se xml.StartElement) IOE.IOEither[fpnet.NetError, T] {
	return IOE.TryCatch(
		func() (T, error) {
			var v T
			return v, dec.DecodeElement(&v, &se)
		},
		wrapXMPP("decode"),
	)
}

// Protocol primitives

func sendStreamHeader(c net.Conn, id string) IOE.IOEither[fpnet.NetError, Void] {
	return writeVoid(c, fmt.Sprintf(
		`<?xml version='1.0'?>`+
			`<stream:stream from='%s' id='%s' version='1.0' xml:lang='en'`+
			` xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams'>`,
		domain, id,
	))
}

func sendSTARTTLSFeature(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return writeVoid(c,
		`<stream:features>`+
			`<starttls xmlns='urn:ietf:params:xml:ns:xmpp-tls'>`+
			`<required/></starttls>`+
			`</stream:features>`,
	)
}

func sendSASLFeatures(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return writeVoid(c,
		`<stream:features>`+
			`<mechanisms xmlns='urn:ietf:params:xml:ns:xmpp-sasl'>`+
			`<mechanism>PLAIN</mechanism>`+
			`</mechanisms>`+
			`</stream:features>`,
	)
}

func sendBindFeatures(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return writeVoid(c,
		`<stream:features>`+
			`<bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/>`+
			`</stream:features>`,
	)
}

// Protocol phases

// starttlsPhase handles the initial stream open, advertises STARTTLS as
// required, reads the client's <starttls/>, sends <proceed/>, performs the TLS
// handshake, and returns a session wrapping the upgraded connection.
func starttlsPhase(c net.Conn, id string, tlsCfg *tls.Config) IOE.IOEither[fpnet.NetError, session] {
	dec := xml.NewDecoder(c)
	return Pipe6(
		readStartEl(dec),
		IOE.Chain(func(_ xml.StartElement) IOE.IOEither[fpnet.NetError, Void] {
			return sendStreamHeader(c, id)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, Void] {
			return sendSTARTTLSFeature(c)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, xml.StartElement] {
			return readStartEl(dec) // consume <starttls/>
		}),
		IOE.Chain(func(_ xml.StartElement) IOE.IOEither[fpnet.NetError, Void] {
			return writeVoid(c, `<proceed xmlns='urn:ietf:params:xml:ns:xmpp-tls'/>`)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, *tls.Conn] {
			return fpnettls.ServerTLS(tlsCfg)(c)
		}),
		IOE.Chain(func(tlsConn *tls.Conn) IOE.IOEither[fpnet.NetError, session] {
			return IOE.Of[fpnet.NetError](session{conn: tlsConn, dec: xml.NewDecoder(tlsConn)})
		}),
	)
}

// saslPhase reads the post-TLS stream open, advertises PLAIN, and
// authenticates the client. Returns the username on success.
func saslPhase(s session, id string) IOE.IOEither[fpnet.NetError, string] {
	return Pipe6(
		readStartEl(s.dec),
		IOE.Chain(func(_ xml.StartElement) IOE.IOEither[fpnet.NetError, Void] {
			return sendStreamHeader(s.conn, id)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, Void] {
			return sendSASLFeatures(s.conn)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, xml.StartElement] {
			return readStartEl(s.dec)
		}),
		IOE.Chain(func(se xml.StartElement) IOE.IOEither[fpnet.NetError, Auth] {
			return decodeEl[Auth](s.dec, se)
		}),
		IOE.Chain(func(auth Auth) IOE.IOEither[fpnet.NetError, string] {
			b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(auth.Value))
			if err != nil {
				return IOE.Left[string](fpnet.NetError{Op: "auth", Err: err})
			}
			parts := strings.SplitN(string(b), "\x00", 3)
			if len(parts) < 3 {
				return IOE.Left[string](fpnet.NetError{Op: "auth", Err: fmt.Errorf("malformed PLAIN auth")})
			}
			return IOE.Of[fpnet.NetError](parts[1])
		}),
		IOE.ChainFirst(func(_ string) IOE.IOEither[fpnet.NetError, Void] {
			return writeVoid(s.conn, `<success xmlns='urn:ietf:params:xml:ns:xmpp-sasl'/>`)
		}),
	)
}

// bindPhase handles stream re-open after auth, bind IQ, and session
// registration. Returns the bare JID on success.
func bindPhase(s session, ref ioref.IORef[ServerState], id, user string) IOE.IOEither[fpnet.NetError, JID] {
	bareJID := fmt.Sprintf("%s@%s", user, domain)
	return Pipe7(
		readStartEl(s.dec),
		IOE.Chain(func(_ xml.StartElement) IOE.IOEither[fpnet.NetError, Void] {
			return sendStreamHeader(s.conn, id)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, Void] {
			return sendBindFeatures(s.conn)
		}),
		IOE.Chain(func(_ Void) IOE.IOEither[fpnet.NetError, xml.StartElement] {
			return readStartEl(s.dec)
		}),
		IOE.Chain(func(se xml.StartElement) IOE.IOEither[fpnet.NetError, BindIQ] {
			return decodeEl[BindIQ](s.dec, se)
		}),
		IOE.ChainFirst(func(iq BindIQ) IOE.IOEither[fpnet.NetError, Void] {
			resource := iq.Resource
			if resource == "" {
				resource = "default"
			}
			reply := fmt.Sprintf(
				`<iq type='result' id='%s'>`+
					`<bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'>`+
					`<jid>%s/%s</jid></bind></iq>`,
				iq.ID, bareJID, resource,
			)
			return writeVoid(s.conn, reply)
		}),
		IOE.ChainFirst(func(_ BindIQ) IOE.IOEither[fpnet.NetError, Void] {
			return IOE.FromIO[fpnet.NetError](ioref.ModifyWithResult(func(st ServerState) pair.Pair[ServerState, Void] {
				st.sessions[bareJID] = s.conn
				return pair.MakePair(st, VOID)
			})(ref))
		}),
		IOE.Chain(func(_ BindIQ) IOE.IOEither[fpnet.NetError, JID] {
			return IOE.Of[fpnet.NetError](bareJID)
		}),
	)
}

func routeMessage(ref ioref.IORef[ServerState], msg Message) {
	toConn := ioref.ModifyWithResult(func(s ServerState) pair.Pair[ServerState, net.Conn] {
		return pair.MakePair(s, s.sessions[msg.To])
	})(ref)()
	if toConn == nil {
		log.Printf("no session for %s, dropping from %s", msg.To, msg.From)
		return
	}
	payload := fmt.Sprintf(
		`<message to='%s' from='%s'><body>%s</body></message>`,
		msg.To, msg.From, msg.Body,
	)
	E.Fold(
		func(err fpnet.NetError) Void {
			log.Printf("delivery error to %s: %v", msg.To, err)
			return VOID
		},
		func(_ int) Void { return VOID },
	)(fpnet.Write([]byte(payload))(toConn)())
}

// stanzaLoop dispatches incoming stanzas until the connection closes,
// then unregisters the session.
func stanzaLoop(s session, ref ioref.IORef[ServerState], jid JID) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		for {
			done := false
			E.Fold(
				func(_ fpnet.NetError) Void {
					done = true
					return VOID
				},
				func(se xml.StartElement) Void {
					switch se.Name.Local {
					case "message":
						E.Fold(
							func(err fpnet.NetError) Void {
								log.Printf("message error from %s: %v (ignoring)", jid, err)
								return VOID
							},
							func(msg Message) Void {
								msg.From = jid
								routeMessage(ref, msg)
								return VOID
							},
						)(decodeEl[Message](s.dec, se)())
					default:
						s.dec.Skip()
					}
					return VOID
				},
			)(readStartEl(s.dec)())
			if done {
				break
			}
		}
		ioref.Modify(func(st ServerState) ServerState {
			delete(st.sessions, jid)
			return st
		})(ref)()
		log.Printf("session closed: %s", jid)
		return E.Right[fpnet.NetError](VOID)
	}
}

// jabberHandler runs the full XMPP session lifecycle for one connection:
// STARTTLS → SASL → bind → stanza loop. The session value threads the
// upgraded TLS connection into all phases after starttlsPhase.
func jabberHandler(ref ioref.IORef[ServerState], tlsCfg *tls.Config) fpnet.Handler {
	return func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
		id := fmt.Sprintf("%d", c.RemoteAddr().(*net.TCPAddr).Port)
		return Pipe1(
			starttlsPhase(c, id, tlsCfg),
			IOE.Chain(func(s session) IOE.IOEither[fpnet.NetError, Void] {
				return Pipe2(
					saslPhase(s, id),
					IOE.Chain(func(user string) IOE.IOEither[fpnet.NetError, JID] {
						return bindPhase(s, ref, id, user)
					}),
					IOE.Chain(func(jid JID) IOE.IOEither[fpnet.NetError, Void] {
						log.Printf("session established: %s", jid)
						return stanzaLoop(s, ref, jid)
					}),
				)
			}),
		)
	}
}

func main() {
	certFile := flag.String("cert", "cert.pem", "TLS certificate file")
	keyFile := flag.String("key", "key.pem", "TLS key file")
	addr := flag.String("addr", ":5222", "listen address")
	flag.Parse()

	tlsCert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal("tls:", err)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	ref := ioref.MakeIORef(newState())()

	Pipe3(
		fpnet.Listen("tcp", *addr),
		IOE.ChainFirst(func(l net.Listener) IOE.IOEither[fpnet.NetError, net.Listener] {
			return IOE.FromIO[fpnet.NetError](func() net.Listener {
				log.Println("XMPP server listening on", *addr)
				return l
			})
		}),
		IOE.Chain(fpnet.Serve(jabberHandler(ref, tlsCfg))),
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
