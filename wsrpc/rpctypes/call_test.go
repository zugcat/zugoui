package rpctypes_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/zugcat/zugoui/observable"
	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

type coded struct{}

func (coded) Error() string          { return "too long" }
func (coded) Code() string           { return "tooLong" }
func (coded) Params() map[string]any { return map[string]any{"max": 3} }

func TestNewError(t *testing.T) {
	assert.Nil(t, rpctypes.NewError(nil))

	assert.Equal(t,
		&rpctypes.Error{Code: string(rpctypes.CodeFailed), Message: "boom"},
		rpctypes.NewError(errors.New("boom")),
	)

	assert.Equal(t,
		&rpctypes.Error{Code: "tooLong", Message: "Title: too long", Params: map[string]any{"max": 3}},
		rpctypes.NewError(fmt.Errorf("Title: %w", coded{})),
	)

	// a failed validation is invalid as a whole, rather than taking the code
	// of whichever field error comes first
	invalid := observable.ValidationError{"Title": coded{}, "Body": coded{}}
	assert.Equal(t, string(rpctypes.CodeInvalid), rpctypes.NewError(invalid).Code)
	assert.Equal(t, string(rpctypes.CodeInvalid),
		rpctypes.NewError(fmt.Errorf("save: %w", invalid)).Code)

	// unless the application gave it a code of its own
	assert.Equal(t, "saveFailed", rpctypes.NewError(wrapped{invalid}).Code)
}

type wrapped struct{ err error }

func (w wrapped) Error() string { return w.err.Error() }
func (w wrapped) Unwrap() error { return w.err }
func (wrapped) Code() string    { return "saveFailed" }
