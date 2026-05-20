package safe_map

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/btree"
)

// Map is a wrapper around btree.Map[string, T] that provides deterministic
// ordering and JSON serialization support. Unlike Go's built-in map[string]T,
// this map maintains keys in sorted order, making it suitable for scenarios
// where consistent ordering is required (e.g., serialization, hashing).
type Map[T any] struct {
	*btree.Map[string, T]
}

// New creates a new Map instance.
func New[T any]() *Map[T] {
	return &Map[T]{
		Map: btree.NewMap[string, T](0),
	}
}

// Has returns true if the key exists in the map.
func (m *Map[T]) Has(key string) bool {
	_, ok := m.Get(key)
	return ok
}

// Keys returns a slice of all keys in sorted order.
func (m *Map[T]) Keys() []string {
	keys := make([]string, 0, m.Len())
	m.Scan(func(key string, _ T) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}

// Values returns a slice of all values in key-sorted order.
func (m *Map[T]) Values() []T {
	values := make([]T, 0, m.Len())
	m.Scan(func(_ string, value T) bool {
		values = append(values, value)
		return true
	})
	return values
}

// Range iterates over all key-value pairs in sorted key order.
// If the function returns false, iteration stops.
func (m *Map[T]) Range(fn func(key string, value T) bool) {
	m.Scan(fn)
}

// Clone creates a shallow copy of the map.
func (m *Map[T]) Clone() *Map[T] {
	clone := New[T]()
	m.Scan(func(key string, value T) bool {
		clone.Set(key, value)
		return true
	})
	return clone
}

// MarshalJSON implements json.Marshaler.
// The map is serialized as a JSON object with keys in sorted order.
func (m *Map[T]) MarshalJSON() ([]byte, error) {
	if m.Len() == 0 {
		return []byte("{}"), nil
	}

	obj := make(map[string]T, m.Len())
	m.Scan(func(key string, value T) bool {
		obj[key] = value
		return true
	})

	return json.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Map[T]) UnmarshalJSON(data []byte) error {
	var obj map[string]T
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if m.Map == nil {
		m.Map = btree.NewMap[string, T](0)
	}
	m.Clear()

	for key, value := range obj {
		m.Set(key, value)
	}

	return nil
}
