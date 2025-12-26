package vii

import "net/http"

type KeyedValidator[T any] interface {
	Validator[T]
	Key() Key[T]
}

func KSV[T any](v KeyedValidator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		return ProvideKey(r, v.Key(), val), nil
	})
}

// KSVOnly stores ONLY by key (does NOT write into the "by type" slot).
func KSVOnly[T any](v KeyedValidator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		return ProvideOnlyKey(r, v.Key(), val), nil
	})
}
