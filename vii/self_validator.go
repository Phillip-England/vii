package vii

import (
	"context"
	"net/http"
	"reflect"
)

type Validator[T any] interface {
	Validate(r *http.Request) (T, error)
}

type AnyValidator interface {
	ValidateAny(r *http.Request) (*http.Request, error)
}

func WrapValidator[T any](v Validator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		return WithValidated(r, val), nil
	})
}

func WrapValidatorKey[T any](k Key[T], v Validator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		r = WithValidated(r, val) // by type
		r = WithValid(r, k, val)  // by key
		return r, nil
	})
}

// WrapValidatorOnlyKey stores ONLY by key (does NOT write into the "by type" slot).
// Use this when you expect multiple instances of the same type in the request.
func WrapValidatorOnlyKey[T any](k Key[T], v Validator[T]) AnyValidator {
	return anyValidatorFunc(func(r *http.Request) (*http.Request, error) {
		val, err := v.Validate(r)
		if err != nil {
			return r, err
		}
		return WithValid(r, k, val), nil
	})
}

type anyValidatorFunc func(r *http.Request) (*http.Request, error)

func (f anyValidatorFunc) ValidateAny(r *http.Request) (*http.Request, error) { return f(r) }

type validatedStoreKey struct{}

type validatedStore struct {
	byType map[reflect.Type]any
	byKey  map[string]any // composite key: type + "|" + name
}

func getStore(r *http.Request) *validatedStore {
	if r == nil {
		return &validatedStore{
			byType: map[reflect.Type]any{},
			byKey:  map[string]any{},
		}
	}
	if v := r.Context().Value(validatedStoreKey{}); v != nil {
		if s, ok := v.(*validatedStore); ok && s != nil {
			if s.byType == nil {
				s.byType = map[reflect.Type]any{}
			}
			if s.byKey == nil {
				s.byKey = map[string]any{}
			}
			return s
		}
	}
	return &validatedStore{
		byType: map[reflect.Type]any{},
		byKey:  map[string]any{},
	}
}

func cloneStore(old *validatedStore) *validatedStore {
	if old == nil {
		return &validatedStore{
			byType: map[reflect.Type]any{},
			byKey:  map[string]any{},
		}
	}
	next := &validatedStore{
		byType: make(map[reflect.Type]any, len(old.byType)+1),
		byKey:  make(map[string]any, len(old.byKey)+1),
	}
	for k, v := range old.byType {
		next.byType[k] = v
	}
	for k, v := range old.byKey {
		next.byKey[k] = v
	}
	return next
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func keyString[T any](k Key[T]) string {
	t := typeOf[T]()
	return t.String() + "|" + k.name
}

func WithValidated[T any](r *http.Request, value T) *http.Request {
	t := typeOf[T]()
	old := getStore(r)
	next := cloneStore(old)
	next.byType[t] = value
	ctx := context.WithValue(r.Context(), validatedStoreKey{}, next)
	return r.WithContext(ctx)
}

func Validated[T any](r *http.Request) (T, bool) {
	var zero T
	t := typeOf[T]()
	s := getStore(r)
	v, ok := s.byType[t]
	if !ok {
		return zero, false
	}
	out, ok := v.(T)
	if !ok {
		return zero, false
	}
	return out, true
}

func WithValid[T any](r *http.Request, k Key[T], value T) *http.Request {
	old := getStore(r)
	next := cloneStore(old)
	next.byKey[keyString(k)] = value
	ctx := context.WithValue(r.Context(), validatedStoreKey{}, next)
	return r.WithContext(ctx)
}

func Valid[T any](r *http.Request, k Key[T]) (T, bool) {
	var zero T
	s := getStore(r)
	v, ok := s.byKey[keyString(k)]
	if !ok {
		return zero, false
	}
	out, ok := v.(T)
	if !ok {
		return zero, false
	}
	return out, true
}

func V[T any](k Key[T], v Validator[T]) AnyValidator {
	return WrapValidatorKey(k, v)
}

func KV[T any](k Key[T], v Validator[T]) AnyValidator {
	return WrapValidatorOnlyKey(k, v)
}

func SV[T any](v Validator[T]) AnyValidator {
	return WrapValidator(v)
}
