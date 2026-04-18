package main

import (
	"bufio"
	"errors"
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
)

type Nick string
type Channel string

type IRCMessage struct {
	Prefix  string
	Command string
	Params  []string
}

// ServerState is a plain value type — mutation is managed by IORef.
type ServerState struct {
	clients  map[Nick]net.Conn
	channels map[Channel][]Nick
	userSet  map[net.Conn]bool
}

func newServerState() ServerState {
	return ServerState{
		clients:  make(map[Nick]net.Conn),
		channels: make(map[Channel][]Nick),
		userSet:  make(map[net.Conn]bool),
	}
}

// cleanup removes all state associated with a connection.
func cleanup(ref ioref.IORef[ServerState], c net.Conn) {
	ioref.Modify(func(s ServerState) ServerState {
		delete(s.userSet, c)
		for nick, conn := range s.clients {
			if conn == c {
				delete(s.clients, nick)
				for ch, members := range s.channels {
					s.channels[ch] = removeNick(members, nick)
				}
				break
			}
		}
		return s
	})(ref)()
}

func removeNick(members []Nick, target Nick) []Nick {
	out := members[:0]
	for _, n := range members {
		if n != target {
			out = append(out, n)
		}
	}
	return out
}

func ircHandler(ref ioref.IORef[ServerState]) fpnet.Handler {
	return func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
		return func() E.Either[fpnet.NetError, Void] {
			log.Printf("new connection from %s", c.RemoteAddr())
			r := bufio.NewReader(c)
			for {
				done := false
				E.Fold(
					func(err fpnet.NetError) Void {
						log.Printf("connection closed from %s: %v", c.RemoteAddr(), err)
						done = true
						return VOID
					},
					func(b []byte) Void {
						E.Fold(
							func(err fpnet.NetError) Void {
								log.Printf("parse error from %s: %v (ignoring)", c.RemoteAddr(), err)
								return VOID
							},
							func(msg IRCMessage) Void {
								log.Printf("recv from %s: %s %v", c.RemoteAddr(), msg.Command, msg.Params)
								E.Fold(
									func(err fpnet.NetError) Void {
										log.Printf("dispatch error from %s: %v", c.RemoteAddr(), err)
										done = true
										return VOID
									},
									func(_ Void) Void { return VOID },
								)(dispatch(ref, c, msg)())
								return VOID
							},
						)(ParseMessage(b))
						return VOID
					},
				)(fpnet.ReadLineFrom(r)())
				if done {
					break
				}
			}
			cleanup(ref, c)
			return E.Right[fpnet.NetError](VOID)
		}
	}
}

// writeVoid writes a message to a connection and discards the byte count.
func writeVoid(c net.Conn, msg []byte) IOE.IOEither[fpnet.NetError, Void] {
	return Pipe1(
		fpnet.Write(msg)(c),
		IOE.Chain(func(_ int) IOE.IOEither[fpnet.NetError, Void] {
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func handleNick(ref ioref.IORef[ServerState], c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	if len(msg.Params) == 0 {
		return IOE.Left[Void](fpnet.NetError{Op: "nick", Err: errors.New("no nickname given")})
	}
	nick := Nick(msg.Params[0])
	return Pipe1(
		IOE.FromIO[fpnet.NetError](ioref.ModifyWithResult(func(s ServerState) pair.Pair[ServerState, bool] {
			s.clients[nick] = c
			return pair.MakePair(s, s.userSet[c])
		})(ref)),
		IOE.Chain(func(userReady bool) IOE.IOEither[fpnet.NetError, Void] {
			if userReady {
				log.Printf("nick registered: %s, sending welcome", nick)
				return sendWelcome(c, nick)
			}
			log.Printf("nick registered: %s, waiting for USER", nick)
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func handleUser(ref ioref.IORef[ServerState], c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return Pipe1(
		IOE.FromIO[fpnet.NetError](ioref.ModifyWithResult(func(s ServerState) pair.Pair[ServerState, Nick] {
			s.userSet[c] = true
			return pair.MakePair(s, nickForState(s, c))
		})(ref)),
		IOE.Chain(func(nick Nick) IOE.IOEither[fpnet.NetError, Void] {
			if nick != "" {
				log.Printf("USER received from %s (nick=%s), sending welcome", c.RemoteAddr(), nick)
				return sendWelcome(c, nick)
			}
			log.Printf("USER received from %s, waiting for NICK", c.RemoteAddr())
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func sendWelcome(c net.Conn, nick Nick) IOE.IOEither[fpnet.NetError, Void] {
	numerics := []IRCMessage{
		{Prefix: "server", Command: "001", Params: []string{string(nick), "Welcome to the IRC server " + string(nick)}},
		{Prefix: "server", Command: "002", Params: []string{string(nick), "Your host is server"}},
		{Prefix: "server", Command: "003", Params: []string{string(nick), "This server was created just now"}},
		{Prefix: "server", Command: "004", Params: []string{string(nick), "server", "0.1", "", ""}},
		{Prefix: "server", Command: "422", Params: []string{string(nick), "MOTD File is missing"}},
	}
	return func() E.Either[fpnet.NetError, Void] {
		for _, m := range numerics {
			log.Printf("sending %s to %s", m.Command, nick)
			if result := writeVoid(c, EncodeMessage(m))(); E.IsLeft(result) {
				return result
			}
		}
		return E.Right[fpnet.NetError](VOID)
	}
}

func handlePing(c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	token := ""
	if len(msg.Params) > 0 {
		token = msg.Params[0]
	}
	log.Printf("PING from %s, sending PONG %s", c.RemoteAddr(), token)
	return writeVoid(c, EncodeMessage(IRCMessage{Command: "PONG", Params: []string{token}}))
}

func handleJoin(ref ioref.IORef[ServerState], c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	if len(msg.Params) == 0 {
		return IOE.Left[Void](fpnet.NetError{Op: "join", Err: errors.New("no channel given")})
	}
	ch := Channel(msg.Params[0])
	return Pipe1(
		IOE.FromIO[fpnet.NetError](ioref.ModifyWithResult(func(s ServerState) pair.Pair[ServerState, Nick] {
			nick := nickForState(s, c)
			s.channels[ch] = append(s.channels[ch], nick)
			return pair.MakePair(s, nick)
		})(ref)),
		IOE.Chain(func(nick Nick) IOE.IOEither[fpnet.NetError, Void] {
			log.Printf("%s joined %s", nick, ch)
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func handlePrivmsg(ref ioref.IORef[ServerState], c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	if len(msg.Params) < 2 {
		return IOE.Left[Void](fpnet.NetError{Op: "privmsg", Err: errors.New("no target or message")})
	}
	ch := Channel(msg.Params[0])
	text := msg.Params[1]
	return Pipe1(
		IOE.FromIO[fpnet.NetError](ioref.ModifyWithResult(func(s ServerState) pair.Pair[ServerState, []net.Conn] {
			nick := nickForState(s, c)
			outMsg := EncodeMessage(IRCMessage{
				Prefix:  string(nick),
				Command: "PRIVMSG",
				Params:  []string{string(ch), text},
			})
			var recipients []net.Conn
			for _, member := range s.channels[ch] {
				if conn, ok := s.clients[member]; ok && conn != c {
					recipients = append(recipients, conn)
				}
			}
			log.Printf("privmsg from %s to %s: %s", nick, ch, text)
			_ = outMsg
			return pair.MakePair(s, recipients)
		})(ref)),
		IOE.Chain(func(recipients []net.Conn) IOE.IOEither[fpnet.NetError, Void] {
			outMsg := EncodeMessage(IRCMessage{
				Prefix:  string(nickForConn(ref, c)),
				Command: "PRIVMSG",
				Params:  []string{string(ch), text},
			})
			for _, conn := range recipients {
				if result := fpnet.Write(outMsg)(conn)(); E.IsLeft(result) {
					log.Printf("write error broadcasting to %s", conn.RemoteAddr())
				}
			}
			return IOE.Of[fpnet.NetError](VOID)
		}),
	)
}

func nickForState(s ServerState, c net.Conn) Nick {
	for nick, conn := range s.clients {
		if conn == c {
			return nick
		}
	}
	return ""
}

func nickForConn(ref ioref.IORef[ServerState], c net.Conn) Nick {
	return nickForState(ioref.Read(ref)(), c)
}

func dispatch(ref ioref.IORef[ServerState], c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	switch msg.Command {
	case "NICK":
		return handleNick(ref, c, msg)
	case "USER":
		return handleUser(ref, c)
	case "PING":
		return handlePing(c, msg)
	case "JOIN":
		return handleJoin(ref, c, msg)
	case "PRIVMSG":
		return handlePrivmsg(ref, c, msg)
	case "QUIT":
		log.Printf("quit from %s", nickForConn(ref, c))
		return fpnet.Close(c)
	default:
		log.Printf("unknown command from %s: %s", c.RemoteAddr(), msg.Command)
		return writeVoid(c, EncodeMessage(IRCMessage{Command: "421", Params: []string{"Unknown command"}}))
	}
}

func main() {
	port := ":6667"
	ref := ioref.MakeIORef(newServerState())()

	Pipe3(
		fpnet.Listen("tcp", port),
		IOE.ChainFirst(func(l net.Listener) IOE.IOEither[fpnet.NetError, net.Listener] {
			return IOE.FromIO[fpnet.NetError](func() net.Listener {
				log.Println("IRC server listening on", port)
				return l
			})
		}),
		IOE.Chain(fpnet.Serve(ircHandler(ref))),
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

// ":nick!user@host PRIVMSG #channel :hello\r\n"

func ParseMessage(b []byte) E.Either[fpnet.NetError, IRCMessage] {
	line := strings.TrimRight(string(b), "\r\n")
	if len(line) == 0 {
		return E.Left[IRCMessage](fpnet.NetError{Op: "parse", Err: errors.New("empty message")})
	}

	msg := IRCMessage{}

	if strings.HasPrefix(line, ":") {
		parts := strings.SplitN(line, " ", 2)
		msg.Prefix = parts[0][1:]
		if len(parts) < 2 {
			return E.Left[IRCMessage](fpnet.NetError{Op: "parse", Err: errors.New("no command")})
		}
		line = parts[1]
	}

	parts := strings.SplitN(line, " :", 2)
	fields := strings.Fields(parts[0])
	if len(fields) == 0 {
		return E.Left[IRCMessage](fpnet.NetError{Op: "parse", Err: errors.New("no command")})
	}

	msg.Command = fields[0]
	msg.Params = fields[1:]

	if len(parts) == 2 {
		msg.Params = append(msg.Params, parts[1])
	}

	return E.Right[fpnet.NetError](msg)
}

func EncodeMessage(m IRCMessage) []byte {
	var sb strings.Builder
	if m.Prefix != "" {
		sb.WriteString(":" + m.Prefix + " ")
	}
	sb.WriteString(m.Command)
	if len(m.Params) > 0 {
		leading := m.Params[:len(m.Params)-1]
		trailing := m.Params[len(m.Params)-1]
		for _, p := range leading {
			sb.WriteString(" " + p)
		}
		if strings.Contains(trailing, " ") {
			sb.WriteString(" :" + trailing)
		} else {
			sb.WriteString(" " + trailing)
		}
	}
	sb.WriteString("\r\n")
	return []byte(sb.String())
}
