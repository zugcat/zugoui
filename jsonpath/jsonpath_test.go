package jsonpath_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zugcat/zugoui/jsonpath"
)

type answer struct {
	Label  string `json:"label"`
	Picked bool   `json:"picked,omitempty"`
}

type panel struct {
	Question string            `json:"question"`
	Answers  []answer          `json:"answers"`
	Address  *address          `json:"address"`
	Extra    map[string]answer `json:"extra"`
	Untagged string
	Hidden   string `json:"-"`
	private  string
}

type address struct {
	City string `json:"city"`
}

func TestKeyPath(t *testing.T) {
	typ := reflect.TypeFor[*panel]()

	for path, want := range map[string]struct {
		key string
		typ reflect.Type
	}{
		"question":         {"Question", reflect.TypeFor[string]()},
		"answers.2.picked": {"Answers.2.Picked", reflect.TypeFor[bool]()},
		"answers.0":        {"Answers.0", reflect.TypeFor[answer]()},
		"address.city":     {"Address.City", reflect.TypeFor[string]()},
		"extra.foo.label":  {"Extra.foo.Label", reflect.TypeFor[string]()},
		"Untagged":         {"Untagged", reflect.TypeFor[string]()},
	} {
		key, ft, err := jsonpath.KeyPath(typ, path)
		require.NoError(t, err, path)
		assert.Equal(t, want.key, key, path)
		assert.Equal(t, want.typ, ft, path)
	}

	for _, path := range []string{"Question", "Hidden", "private", "answers.x", "question.length", "nope"} {
		_, _, err := jsonpath.KeyPath(typ, path)
		assert.ErrorIs(t, err, jsonpath.ErrNoField, path)
	}
}

func TestFromKeyPath(t *testing.T) {
	typ := reflect.TypeFor[panel]()

	assert.Equal(t, "answers.2.picked", jsonpath.FromKeyPath(typ, "Answers.2.Picked"))
	assert.Equal(t, "address.city", jsonpath.FromKeyPath(typ, "Address.City"))
	assert.Equal(t, "Untagged", jsonpath.FromKeyPath(typ, "Untagged"))
	assert.Equal(t, "Nope.Deeper", jsonpath.FromKeyPath(typ, "Nope.Deeper"))
}
