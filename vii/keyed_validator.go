package vii

import "net/http"

// KeyedValidator is a self-contained validator that also exposes its own key.
// This guarantees the validated value is always accessible "with itself".
type KeyedValidator[T any] interface {
	Validator[T]
	Key() Key[T]
}

// KSV wraps a KeyedValidator and stores the validated value by BOTH:
//   - type (Validated[T])
//   - key  (Valid[T](..., v.Key()))
func KSV[T any](v KeyedValidator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		return ProvideKey(r, v.Key(), val), nil
	})
}
