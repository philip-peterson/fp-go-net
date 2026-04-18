// Package fpgonet provides functional-programming-style wrappers around the
// standard library's net package, exposing network operations as IOEither
// values for principled, composable error handling.
package fpgonet

// NetError wraps a network operation name and its underlying error.
type NetError struct {
	// Op is the name of the network operation that failed (e.g. "dial", "read").
	Op string
	// Err is the underlying error returned by the operation.
	Err error
}

// Error returns a string combining the operation name and the underlying error message.
func (e NetError) Error() string { return e.Op + ": " + e.Err.Error() }

func wrapErr(op string) func(error) NetError {
	return func(err error) NetError { return NetError{op, err} }
}
