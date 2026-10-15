// Package jsonpath maps between the key paths zugoui's observables use, made of
// Go field names, and the paths browser code uses, made of the names
// encoding/json would give the same fields.
//
// A path is dotted, with slice and array indices and map keys as their own
// segments: "Answers.2.Picked" is "answers.2.picked" for a struct whose fields
// are tagged `json:"answers"` and `json:"picked"`. A field with no json tag
// keeps its Go name, so for an untagged struct both paths are the same.
package jsonpath

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var ErrNoField = errors.New("no such field")

// Name returns the name encoding/json gives a struct field, and whether it is
// encoded at all.
func Name(f reflect.StructField) (name string, ok bool) {
	if !f.IsExported() {
		return "", false
	}
	tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	switch tag {
	case "-":
		return "", false
	case "":
		return f.Name, true
	}
	return tag, true
}

func deref(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// KeyPath returns the Go key path for a JSON path into a value of type t, and
// the type of the value it reaches.
func KeyPath(t reflect.Type, path string) (string, reflect.Type, error) {
	segments := strings.Split(path, ".")
	keys := make([]string, 0, len(segments))

	for _, segment := range segments {
		t = deref(t)
		if t == nil {
			return "", nil, fmt.Errorf("%s: %w", path, ErrNoField)
		}

		switch t.Kind() {
		case reflect.Struct:
			found := false
			for i := range t.NumField() {
				f := t.Field(i)
				if name, ok := Name(f); ok && name == segment {
					keys = append(keys, f.Name)
					t = f.Type
					found = true
					break
				}
			}
			if !found {
				return "", nil, fmt.Errorf("%s: %q: %w", path, segment, ErrNoField)
			}

		case reflect.Slice, reflect.Array:
			if _, err := strconv.Atoi(segment); err != nil {
				return "", nil, fmt.Errorf("%s: %q is not an index: %w", path, segment, ErrNoField)
			}
			keys = append(keys, segment)
			t = t.Elem()

		case reflect.Map:
			keys = append(keys, segment)
			t = t.Elem()

		default:
			return "", nil, fmt.Errorf("%s: %q: %w", path, segment, ErrNoField)
		}
	}

	return strings.Join(keys, "."), t, nil
}

// FromKeyPath returns the JSON path for a Go key path into a value of type t.
// A segment it can't resolve is kept as it is, as are the ones after it.
func FromKeyPath(t reflect.Type, keyPath string) string {
	segments := strings.Split(keyPath, ".")

	for n, segment := range segments {
		t = deref(t)
		if t == nil {
			break
		}

		switch t.Kind() {
		case reflect.Struct:
			f, ok := t.FieldByName(segment)
			if !ok {
				return strings.Join(segments, ".")
			}
			if name, ok := Name(f); ok {
				segments[n] = name
			}
			t = f.Type

		case reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()

		default:
			return strings.Join(segments, ".")
		}
	}

	return strings.Join(segments, ".")
}
