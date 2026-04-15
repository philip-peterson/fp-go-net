package main

import (
	"bytes"
	"net"
	"testing"
	"time"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"

	fpnet "github.com/philip-peterson/fp-go-net"
)

// mockConn implements net.Conn, capturing writes in a buffer.
type mockConn struct {
	buf    bytes.Buffer
	closed bool
}

func (m *mockConn) Write(b []byte) (int, error)        { return m.buf.Write(b) }
func (m *mockConn) Read(b []byte) (int, error)         { return 0, nil }
func (m *mockConn) Close() error                       { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) written() string { return m.buf.String() }

// mustRight fails the test if the Either is a Left.
func mustRight[A any](t *testing.T, result E.Either[fpnet.NetError, A]) A {
	t.Helper()
	var val A
	E.Fold(
		func(err fpnet.NetError) Void {
			t.Fatalf("expected Right, got Left: %v", err)
			return VOID
		},
		func(a A) Void {
			val = a
			return VOID
		},
	)(result)
	return val
}

func newState() *ServerState {
	return &ServerState{
		clients:  make(map[Nick]net.Conn),
		channels: make(map[Channel][]Nick),
		userSet:  make(map[net.Conn]bool),
	}
}

// --- ParseMessage ---

func TestParseMessage_Basic(t *testing.T) {
	msg := mustRight(t, ParseMessage([]byte("PING :token\r\n")))
	if msg.Command != "PING" {
		t.Errorf("command = %q, want PING", msg.Command)
	}
	if len(msg.Params) != 1 || msg.Params[0] != "token" {
		t.Errorf("params = %v, want [token]", msg.Params)
	}
}

func TestParseMessage_WithPrefix(t *testing.T) {
	msg := mustRight(t, ParseMessage([]byte(":nick!user@host PRIVMSG #chan :hello world\r\n")))
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
	conn := &mockConn{}
	mustRight(t, handlePing(conn, IRCMessage{Command: "PING", Params: []string{"token123"}})())
	want := "PONG token123\r\n"
	if conn.written() != want {
		t.Errorf("got %q, want %q", conn.written(), want)
	}
}

// --- registration ---

func TestRegistration_NickThenUser(t *testing.T) {
	conn := &mockConn{}
	state := newState()

	mustRight(t, handleNick(state, conn, IRCMessage{Params: []string{"alice"}})())
	if conn.written() != "" {
		t.Errorf("expected no output before USER, got %q", conn.written())
	}

	mustRight(t, handleUser(state, conn)())
	out := conn.written()
	if out == "" {
		t.Fatal("expected welcome sequence, got nothing")
	}
	firstLine := out[:len(":server 001 alice Welcome to the IRC server alice\r\n")]
	if E.IsLeft(ParseMessage([]byte(firstLine))) {
		t.Fatal("could not parse first response line")
	}
}

func TestRegistration_UserThenNick(t *testing.T) {
	conn := &mockConn{}
	state := newState()

	mustRight(t, handleUser(state, conn)())
	if conn.written() != "" {
		t.Errorf("expected no output before NICK, got %q", conn.written())
	}

	mustRight(t, handleNick(state, conn, IRCMessage{Params: []string{"bob"}})())
	if conn.written() == "" {
		t.Error("expected welcome sequence after NICK, got nothing")
	}
}

func TestHandleNick_NoParams(t *testing.T) {
	conn := &mockConn{}
	result := handleNick(newState(), conn, IRCMessage{Params: []string{}})()
	if !E.IsLeft(result) {
		t.Error("expected Left for missing nick param")
	}
}

func TestUnknownCommand(t *testing.T) {
	conn := &mockConn{}
	mustRight(t, dispatch(newState(), conn, IRCMessage{Command: "FOOBAR"})())
	if conn.written() == "" {
		t.Error("expected 421 response, got nothing")
	}
	msg := mustRight(t, ParseMessage([]byte(conn.written())))
	if msg.Command != "421" {
		t.Errorf("command = %q, want 421", msg.Command)
	}
}

func TestCleanup_RemovesClientFromState(t *testing.T) {
	conn := &mockConn{}
	state := newState()

	mustRight(t, handleNick(state, conn, IRCMessage{Params: []string{"carol"}})())
	mustRight(t, handleUser(state, conn)())

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
