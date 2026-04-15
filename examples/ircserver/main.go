package main

import (
	"bufio"
	"errors"
	"log"
	"net"
	"strings"
	"sync"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"

	fpnet "github.com/philip-peterson/fp-go-net"
)

type Nick string
type Channel string

type IRCMessage struct {
	Prefix  string
	Command string
	Params  []string
}

type ServerState struct {
	clients  map[Nick]net.Conn
	channels map[Channel][]Nick
	userSet  map[net.Conn]bool // connection has sent USER
	mu       sync.RWMutex
}

func (s *ServerState) cleanup(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func ircHandler(state *ServerState) fpnet.Handler {
	return func(c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
		return func() E.Either[fpnet.NetError, Void] {
			log.Printf("new connection from %s", c.RemoteAddr())
			r := bufio.NewReader(c)
			for {
				stop := false
				E.Fold(
					func(err fpnet.NetError) Void {
						log.Printf("connection closed from %s: %v", c.RemoteAddr(), err)
						stop = true
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
								dispatch(state, c, msg)()
								return VOID
							},
						)(ParseMessage(b))
						return VOID
					},
				)(fpnet.ReadLineFrom(r)())
				if stop {
					break
				}
			}
			state.cleanup(c)
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

func handleNick(state *ServerState, c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		if len(msg.Params) == 0 {
			return E.Left[Void](fpnet.NetError{Op: "nick", Err: errors.New("no nickname given")})
		}
		nick := Nick(msg.Params[0])
		state.mu.Lock()
		state.clients[nick] = c
		userReady := state.userSet[c]
		state.mu.Unlock()
		if userReady {
			log.Printf("nick registered: %s, sending welcome", nick)
			return sendWelcome(c, nick)()
		}
		log.Printf("nick registered: %s, waiting for USER", nick)
		return E.Right[fpnet.NetError](VOID)
	}
}

func handleUser(state *ServerState, c net.Conn) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		state.mu.Lock()
		state.userSet[c] = true
		nick := nickForConnLocked(state, c)
		state.mu.Unlock()
		if nick != "" {
			log.Printf("USER received from %s (nick=%s), sending welcome", c.RemoteAddr(), nick)
			return sendWelcome(c, nick)()
		}
		log.Printf("USER received from %s, waiting for NICK", c.RemoteAddr())
		return E.Right[fpnet.NetError](VOID)
	}
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
			c.Write(EncodeMessage(m))
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

func handleJoin(state *ServerState, c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		if len(msg.Params) == 0 {
			return E.Left[Void](fpnet.NetError{Op: "join", Err: errors.New("no channel given")})
		}
		ch := Channel(msg.Params[0])
		nick := nickForConn(state, c)
		state.mu.Lock()
		state.channels[ch] = append(state.channels[ch], nick)
		state.mu.Unlock()
		log.Printf("%s joined %s", nick, ch)
		return E.Right[fpnet.NetError](VOID)
	}
}

func handlePrivmsg(state *ServerState, c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	return func() E.Either[fpnet.NetError, Void] {
		if len(msg.Params) < 2 {
			return E.Left[Void](fpnet.NetError{Op: "privmsg", Err: errors.New("no target or message")})
		}
		ch := Channel(msg.Params[0])
		text := msg.Params[1]
		nick := nickForConn(state, c)
		log.Printf("privmsg from %s to %s: %s", nick, ch, text)
		outMsg := EncodeMessage(IRCMessage{
			Prefix:  string(nick),
			Command: "PRIVMSG",
			Params:  []string{string(ch), text},
		})
		state.mu.RLock()
		var recipients []net.Conn
		for _, member := range state.channels[ch] {
			if conn, ok := state.clients[member]; ok && conn != c {
				recipients = append(recipients, conn)
			}
		}
		state.mu.RUnlock()
		for _, conn := range recipients {
			fpnet.Write(outMsg)(conn)()
		}
		return E.Right[fpnet.NetError](VOID)
	}
}

func nickForConn(state *ServerState, c net.Conn) Nick {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return nickForConnLocked(state, c)
}

// nickForConnLocked looks up the nick for a connection; caller must hold mu.
// Returns empty string if not yet registered.
func nickForConnLocked(state *ServerState, c net.Conn) Nick {
	for nick, conn := range state.clients {
		if conn == c {
			return nick
		}
	}
	return ""
}

func dispatch(state *ServerState, c net.Conn, msg IRCMessage) IOE.IOEither[fpnet.NetError, Void] {
	switch msg.Command {
	case "NICK":
		return handleNick(state, c, msg)
	case "USER":
		return handleUser(state, c)
	case "PING":
		return handlePing(c, msg)
	case "JOIN":
		return handleJoin(state, c, msg)
	case "PRIVMSG":
		return handlePrivmsg(state, c, msg)
	case "QUIT":
		log.Printf("quit from %s", nickForConn(state, c))
		return fpnet.Close(c)
	default:
		log.Printf("unknown command from %s: %s", c.RemoteAddr(), msg.Command)
		return writeVoid(c, EncodeMessage(IRCMessage{Command: "421", Params: []string{"Unknown command"}}))
	}
}

func main() {
	state := &ServerState{
		clients:  make(map[Nick]net.Conn),
		channels: make(map[Channel][]Nick),
		userSet:  make(map[net.Conn]bool),
	}

	log.Println("IRC server listening on :6667")
	Pipe1(
		fpnet.Listen("tcp", ":6667"),
		IOE.Chain(fpnet.Serve(ircHandler(state))),
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
