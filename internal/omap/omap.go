// Package omap is a string-keyed map that keeps insertion order, like a Dart map literal.
package omap

// Map keeps keys in insertion order; setting an existing key keeps its position.
// The zero value is not usable; call New.
type Map[V any] struct {
	keys   []string
	values map[string]V
}

// New returns an empty map.
func New[V any]() *Map[V] {
	return &Map[V]{values: map[string]V{}}
}

// Get returns the value for key and whether the key is present.
func (m *Map[V]) Get(key string) (V, bool) {
	value, ok := m.values[key]
	return value, ok
}

// Value returns the value for key, or the zero value when it is absent.
func (m *Map[V]) Value(key string) V {
	return m.values[key]
}

// Has reports whether key is present.
func (m *Map[V]) Has(key string) bool {
	_, ok := m.values[key]
	return ok
}

// Set adds or replaces key; a new key goes last.
func (m *Map[V]) Set(key string, value V) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes key when present.
func (m *Map[V]) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i:i], m.keys[i+1:]...)
			break
		}
	}
}

// Keys returns the keys in insertion order; the slice is a copy.
func (m *Map[V]) Keys() []string {
	return append([]string(nil), m.keys...)
}

// Len returns the number of keys.
func (m *Map[V]) Len() int {
	return len(m.keys)
}

// Copy returns a shallow copy with the same order.
func (m *Map[V]) Copy() *Map[V] {
	out := New[V]()
	for _, key := range m.keys {
		out.Set(key, m.values[key])
	}
	return out
}

// Of builds a map from alternating key and value arguments, in order.
func Of[V any](pairs ...any) *Map[V] {
	out := New[V]()
	for i := 0; i+1 < len(pairs); i += 2 {
		var value V
		if pairs[i+1] != nil {
			value = pairs[i+1].(V)
		}
		out.Set(pairs[i].(string), value)
	}
	return out
}
