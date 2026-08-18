package observable

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ModelValidator validates the fields set in any type backed by a model
type ModelValidator interface {
	// ValidateModel returns a map of keys (of which a Source can see) and their errors.
	ValidateModel() error
}

// ValidationError contains all errors associated with a field
type ValidationError map[string]error

func (v ValidationError) Error() string {
	errs := make([]error, 0, len(v))
	for k, v := range v {
		errs = append(errs, fmt.Errorf("%s: %w", k, v))
	}
	return errors.Join(errs...).Error()
}

func (v ValidationError) Unwrap() []error {
	return slices.Collect(maps.Values(v))
}

// ValidateSource calls ValidateModel on the underlying model,
// recursively if there are keys with ModelValidator values.
// ValidateSource returns a ValidationError if there are errors present
func ValidateSource(s Source) error {
	model := s.Model()
	if !model.IsValid() {
		return nil
	}

	result := make(ValidationError)
	mv, ok := model.Interface().(ModelValidator)
	if ok {
		err := mv.ValidateModel()
		if errors, ok := errors.AsType[ValidationError](err); ok {
			result = errors
		} else if err != nil {
			return err
		}
	}

	for _, key := range s.Keys() {
		if s, ok := s.Value(key).(Source); ok {
			if err := ValidateSource(s); err != nil {
				if keyResult, ok := errors.AsType[ValidationError](err); ok {
					if len(keyResult) == 0 {
						continue
					}
					for k, v := range keyResult {
						result[key+"."+k] = v
					}
				} else {
					result[key] = err
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
