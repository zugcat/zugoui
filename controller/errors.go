package controller

// ActionError returns err with a code for the browser, which looks up a
// translated message by it and falls back to err's text. Wrapping err keeps it
// reachable with errors.Is and errors.As.
func ActionError(code string, err error) error {
	return &actionError{code: code, err: err}
}

type actionError struct {
	code string
	err  error
}

func (e *actionError) Error() string { return e.err.Error() }
func (e *actionError) Unwrap() error { return e.err }
func (e *actionError) Code() string  { return e.code }
