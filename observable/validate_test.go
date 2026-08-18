package observable_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CCorderZugcat/zugoui/observable"
	"github.com/CCorderZugcat/zugoui/observable/controllers"
)

type myModel struct {
	Field1   string
	Field2   int
	Submodel *submodel
}

type submodel struct {
	Field1 int
}

var errRange = errors.New("range")
var errEmpty = errors.New("empty")

func (m myModel) ValidateModel() error {
	result := make(observable.ValidationError)

	if m.Field1 == "" {
		result["Field1"] = fmt.Errorf("%w: Field1 must not be empty", errEmpty)
	}
	if m.Field2 < 0 {
		result["Field2"] = fmt.Errorf("%w: Field2 must be greater than or equal to 0", errRange)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (s submodel) ValidateModel() error {
	result := make(observable.ValidationError)

	if s.Field1 == 0 {
		result["Field1"] = fmt.Errorf("%w: Field1 must not be zero", errRange)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func TestValidate(t *testing.T) {
	m := &myModel{
		Submodel: &submodel{},
	}

	s := controllers.New(m)
	err := observable.ValidateSource(s)
	require.Error(t, err)
	t.Logf("%v (expected)", err)
	assert.ErrorIs(t, err, errEmpty)
	assert.ErrorIs(t, err, errRange)

	ve := err.(observable.ValidationError)
	assert.ErrorIs(t, ve["Field1"], errEmpty)
	assert.ErrorIs(t, ve["Submodel.Field1"], errRange)
}

type bar struct {
	Field string
	err   error
}

func (b bar) ValidateModel() error { return b.err }

type foo struct {
	Bar bar
}

func (f foo) ValidateModel() error { return nil }

func TestRecursiveValidate(t *testing.T) {
	f := &foo{}
	s := controllers.New(f)
	assert.NoError(t, observable.ValidateSource(s))

	errNope := errors.New("nope")
	f.Bar.err = errNope
	err := observable.ValidateSource(s)
	assert.ErrorIs(t, err, errNope)

	var errs observable.ValidationError
	require.ErrorAs(t, err, &errs)
	assert.ErrorIs(t, errs["Bar"], errNope)
}
