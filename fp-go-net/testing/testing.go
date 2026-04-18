// Package testing provides test helpers for fp-go-net.
package testing

import (
	"testing"

	E "github.com/IBM/fp-go/v2/either"
	. "github.com/IBM/fp-go/v2/function"

	fpnet "github.com/philip-peterson/fp-go-net"
)

// AssertRight fails the test if result is a Left, and returns the Right value.
func AssertRight[A any](t *testing.T, result E.Either[fpnet.NetError, A]) A {
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
