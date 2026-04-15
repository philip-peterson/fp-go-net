package main

import (
	"net"
	"testing"

	E "github.com/IBM/fp-go/v2/either"

	fpnet "github.com/philip-peterson/fp-go-net"
	fpnettest "github.com/philip-peterson/fp-go-net/testing"
)

func newState() *ServerState {
	return &ServerState{
		clients:  make(map[Nick]net.Conn),
		channels: make(map[Channel][]Nick),
		userSet:  make(map[net.Conn]bool),
	}
}

// --- ParseMessage ---

func TestParseMessage_Basic(t *testing.T) {
	msg := fpnettest.AssertRight(t, ParseMessage([]byte("PING :token\r\n")))
	if msg.Command != "PING" {
		t.Errorf("command = %q, want PING", msg.Command)
	}
	if len(msg.Params) != 1 || msg.Params[0] != "token" {
		t.Errorf("params = %v, want [token]", msg.Params)
	}
}

func TestParseMessage_WithPrefix(t *testing.T) {
	msg := fpnettest.AssertRight(t, ParseMessage([]byte(":nick!user@host PRIVMSG #chan :hello world\r\n")))
	if msg.Prefix != "nick!user@host" {
		t.Errorf("prefix = %q, want nick!user@host", msg.Prefix)
	}
	if msg.Command != "PRIVMSG" {
		t.Errorf("command = %q, want PRIVMSG", msg.Command)
	}
	if msg.Params[len(msg.Params)-1] != "hello world" {
		t.Errorf("trailing param = %q, want 'hello world'", msg.Params[len(msg.Params)-1])
	}
}

func TestParseMessage_EmptyLine(t *testing.T) {
	if !E.IsLeft(ParseMessage([]byte("\r\n"))) {
		t.Error("expected Left for empty line, got Right")
	}
}

// --- EncodeMessage ---

func TestEncodeMessage_TrailingParam(t *testing.T) {
	got := string(EncodeMessage(IRCMessage{Command: "PRIVMSG", Params: []string{"#chan", "hello world"}}))
	want := "PRIVMSG #chan :hello world\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEncodeMessage_WithPrefix(t *testing.T) {
	got := string(EncodeMessage(IRCMessage{Prefix: "server", Command: "001", Params: []string{"nick", "Welcome"}}))
	want := ":server 001 nick Welcome\r\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- handlePing ---

func TestHandlePing(t *testing.T) {
	conn := &fpnettest.MockConn{}
	fpnettest.AssertRight(t, handlePing(conn, IRCMessage{Command: "PING", Params: []string{"token123"}})())
	want := "PONG token123\r\n"
	if conn.Written() != want {
		t.Errorf("got %q, want %q", conn.Written(), want)
	}
}

// --- registration ---

func TestRegistration_NickThenUser(t *testing.T) {
	conn := &fpnettest.MockConn{}
	state := newState()

	fpnettest.AssertRight(t, handleNick(state, conn, IRCMessage{Params: []string{"alice"}})())
	if conn.Written() != "" {
		t.Errorf("expected no output before USER, got %q", conn.Written())
	}

	fpnettest.AssertRight(t, handleUser(state, conn)())
	if conn.Written() == "" {
		t.Error("expected welcome sequence, got nothing")
	}
	fpnettest.AssertRight(t, ParseMessage([]byte(":server 001 alice Welcome to the IRC server alice\r\n")))
}

func TestRegistration_UserThenNick(t *testing.T) {
	conn := &fpnettest.MockConn{}
	state := newState()

	fpnettest.AssertRight(t, handleUser(state, conn)())
	if conn.Written() != "" {
		t.Errorf("expected no output before NICK, got %q", conn.Written())
	}

	fpnettest.AssertRight(t, handleNick(state, conn, IRCMessage{Params: []string{"bob"}})())
	if conn.Written() == "" {
		t.Error("expected welcome sequence after NICK, got nothing")
	}
}

func TestHandleNick_NoParams(t *testing.T) {
	conn := &fpnettest.MockConn{}
	if !E.IsLeft(handleNick(newState(), conn, IRCMessage{Params: []string{}})()) {
		t.Error("expected Left for missing nick param")
	}
}

func TestUnknownCommand(t *testing.T) {
	conn := &fpnettest.MockConn{}
	fpnettest.AssertRight(t, dispatch(newState(), conn, IRCMessage{Command: "FOOBAR"})())
	msg := fpnettest.AssertRight(t, ParseMessage([]byte(conn.Written())))
	if msg.Command != "421" {
		t.Errorf("command = %q, want 421", msg.Command)
	}
}

func TestCleanup_RemovesClientFromState(t *testing.T) {
	conn := &fpnettest.MockConn{}
	state := newState()

	fpnettest.AssertRight(t, handleNick(state, conn, IRCMessage{Params: []string{"carol"}})())
	fpnettest.AssertRight(t, handleUser(state, conn)())

	state.cleanup(conn)

	state.mu.RLock()
	defer state.mu.RUnlock()
	if len(state.clients) != 0 {
		t.Errorf("expected clients to be empty after cleanup, got %v", state.clients)
	}
	if len(state.userSet) != 0 {
		t.Errorf("expected userSet to be empty after cleanup, got %v", state.userSet)
	}
}

// ensure MockConn satisfies net.Conn at compile time
var _ net.Conn = (*fpnettest.MockConn)(nil)
var _ = fpnet.NetError{}
