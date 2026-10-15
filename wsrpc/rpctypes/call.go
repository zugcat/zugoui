package rpctypes

import (
	"errors"
	"maps"

	"github.com/zugcat/zugoui/observable"
)

// CallReq Call request: an action called by name, with its arguments as JSON
type CallReq struct {
	Action string // action name
	Args   []byte // JSON arguments, empty for none
}

// CallRes Call result
type CallRes struct {
	Value []byte // JSON of the action's result, when Error is nil
	Error *Error // the action's failure, if it failed
}

// Error is an application error as the browser receives it: a code to look up
// a translated message by, the English message as a fallback, and the values a
// translated message needs.
type Error struct {
	Code    string
	Message string
	Params  map[string]any
}

type codedError interface {
	error
	Code() string
}

// NewError returns err as an *Error. The code and params come from the first
// error in err's chain with a Code() string method, and from its
// Params() map[string]any method if it has one. Without one, the code is
// CodeFailed, or CodeInvalid for a failed validation: the field errors inside
// an [observable.ValidationError] carry codes of their own, and any one of
// them would otherwise stand for the whole call.
func NewError(err error) *Error {
	if err == nil {
		return nil
	}

	e := &Error{Code: string(CodeFailed), Message: err.Error()}

	if invalid(err) {
		e.Code = string(CodeInvalid)
	} else if coded, ok := errors.AsType[codedError](err); ok && coded.Code() != "" {
		e.Code = coded.Code()
		if p, ok := coded.(interface{ Params() map[string]any }); ok {
			e.Params = maps.Clone(p.Params())
		}
	}

	return e
}

// invalid reports whether err's chain reaches an [observable.ValidationError]
// before any error with a code of its own.
func invalid(err error) bool {
	for ; err != nil; err = errors.Unwrap(err) {
		if _, ok := err.(observable.ValidationError); ok {
			return true
		}
		if coded, ok := err.(codedError); ok && coded.Code() != "" {
			return false
		}
	}
	return false
}
