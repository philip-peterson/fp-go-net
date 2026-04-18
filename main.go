package fpgonet


type NetError struct {
	Op  string
	Err error
}

func (e NetError) Error() string { return e.Op + ": " + e.Err.Error() }

func wrapErr(op string) func(error) NetError {
	return func(err error) NetError { return NetError{op, err} }
}
