package safe_map

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/btree"
)

// Map is a wrapper around btree.Map[string, any] that provides deterministic
// ordering and JSON serialization support. Unlike Go's built-in map[string]any,
// this map maintains keys in sorted order, making it suitable for scenarios
// where consistent ordering is required (e.g., serialization, hashing).
type Map struct {
	*btree.Map[string, any]
}

// New creates a new Map instance.
func New() *Map {
	return &Map{
		Map: btree.NewMap[string, any](0),
	}
}

// Has returns true if the key exists in the map.
func (m *Map) Has(key string) bool {
	_, ok := m.Get(key)
	return ok
}

// Keys returns a slice of all keys in sorted order.
func (m *Map) Keys() []string {
	keys := make([]string, 0, m.Len())
	m.Scan(func(key string, _ any) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}

// Values returns a slice of all values in key-sorted order.
func (m *Map) Values() []any {
	values := make([]any, 0, m.Len())
	m.Scan(func(_ string, value any) bool {
		values = append(values, value)
		return true
	})
	return values
}

// Range iterates over all key-value pairs in sorted key order.
// If the function returns false, iteration stops.
func (m *Map) Range(fn func(key string, value any) bool) {
	m.Scan(fn)
}

// Clone creates a shallow copy of the map.
func (m *Map) Clone() *Map {
	clone := New()
	m.Scan(func(key string, value any) bool {
		clone.Set(key, value)
		return true
	})
	return clone
}

// MarshalJSON implements json.Marshaler.
// The map is serialized as a JSON object with keys in sorted order.
func (m *Map) MarshalJSON() ([]byte, error) {
	if m.Len() == 0 {
		return []byte("{}"), nil
	}

	obj := make(map[string]any, m.Len())
	m.Scan(func(key string, value any) bool {
		obj[key] = value
		return true
	})

	return json.Marshal(obj)
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Map) UnmarshalJSON(data []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if m.Map == nil {
		m.Map = btree.NewMap[string, any](0)
	}
	m.Clear()

	for key, value := range obj {
		m.Set(key, value)
	}

	return nil
}
