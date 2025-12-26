package vii

// Key is a typed name-based key.
// Keys are equal by (type T + name). This means you can recreate keys anywhere
// without needing a shared global variable, and retrieval still works.
type Key[T any] struct {
	name string
}

// NewKey creates a typed key with a human-readable name.
// The name participates in uniqueness alongside T.
func NewKey[T any](name string) Key[T] {
	return Key[T]{name: name}
}

func (k Key[T]) Name() string { return k.name }
