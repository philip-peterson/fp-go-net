package fp_go_net

import (
	"net"
	"testing"

	. "github.com/IBM/fp-go/v2/function"

	E "github.com/IBM/fp-go/v2/either"
)

type MockListener struct{}

var _ net.Listener = MockListener{}

func (l MockListener) Accept() (net.Conn, error) {
	return nil, nil
}

func (l MockListener) Close() error {
	return nil
}

func (l MockListener) Addr() net.Addr {
	return nil
}

func TestListen_Success(t *testing.T) {
	mockListener := &MockListener{} // implements net.Listener
	mockFn := func(network, addr string) (net.Listener, error) {
		return mockListener, nil
	}

	result := listenInternal(mockFn, "tcp", ":8080")()

	E.Fold(
		func(err NetError) Void {
			t.Fatalf("expected Right, got Left: %v", err)
			return VOID
		},
		func(l net.Listener) Void {
			/* assert l == mockListener */
			// assert.Equal(t, l, b)
			return VOID
		},
	)(result)
}
