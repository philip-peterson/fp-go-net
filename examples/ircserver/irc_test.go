package main

import (
	"net"
	"testing"

	E "github.com/IBM/fp-go/v2/either"
	"github.com/IBM/fp-go/v2/ioref"

	fpnet "github.com/philip-peterson/fp-go-net"
	fpnettest "github.com/philip-peterson/fp-go-net/testing"
)

func newState() ioref.IORef[ServerState] {
	return ioref.MakeIORef(newServerState())()
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
	ref := newState()

	fpnettest.AssertRight(t, handleNick(ref, conn, IRCMessage{Params: []string{"alice"}})())
	if conn.Written() != "" {
		t.Errorf("expected no output before USER, got %q", conn.Written())
	}

	fpnettest.AssertRight(t, handleUser(ref, conn)())
	if conn.Written() == "" {
		t.Error("expected welcome sequence, got nothing")
	}
	fpnettest.AssertRight(t, ParseMessage([]byte(":server 001 alice Welcome to the IRC server alice\r\n")))
}

func TestRegistration_UserThenNick(t *testing.T) {
	conn := &fpnettest.MockConn{}
	ref := newState()

	fpnettest.AssertRight(t, handleUser(ref, conn)())
	if conn.Written() != "" {
		t.Errorf("expected no output before NICK, got %q", conn.Written())
	}

	fpnettest.AssertRight(t, handleNick(ref, conn, IRCMessage{Params: []string{"bob"}})())
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
	ref := newState()

	fpnettest.AssertRight(t, handleNick(ref, conn, IRCMessage{Params: []string{"carol"}})())
	fpnettest.AssertRight(t, handleUser(ref, conn)())

	cleanup(ref, conn)

	s := ioref.Read(ref)()
	if len(s.clients) != 0 {
		t.Errorf("expected clients to be empty after cleanup, got %v", s.clients)
	}
	if len(s.userSet) != 0 {
		t.Errorf("expected userSet to be empty after cleanup, got %v", s.userSet)
	}
}

// ensure MockConn satisfies net.Conn at compile time
var _ net.Conn = (*fpnettest.MockConn)(nil)
var _ = fpnet.NetError{}
