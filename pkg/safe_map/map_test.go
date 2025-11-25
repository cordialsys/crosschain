package safe_map_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cordialsys/crosschain/pkg/safe_map"
)

type Map = safe_map.Map

func ToMap(m *Map) map[string]any {
	result := make(map[string]any, m.Len())
	m.Scan(func(key string, value any) bool {
		result[key] = value
		return true
	})
	return result
}

func FromMap(data map[string]any) *Map {
	m := safe_map.New()
	for key, value := range data {
		m.Set(key, value)
	}
	return m
}

func TestMap_SetAndGet(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     any
		wantOk    bool
		getKey    string
		wantVal   any
		wantGetOk bool
	}{
		{
			name:      "string value",
			key:       "name",
			value:     "Alice",
			getKey:    "name",
			wantVal:   "Alice",
			wantGetOk: true,
		},
		{
			name:      "int value",
			key:       "age",
			value:     42,
			getKey:    "age",
			wantVal:   42,
			wantGetOk: true,
		},
		{
			name:      "float value",
			key:       "price",
			value:     99.99,
			getKey:    "price",
			wantVal:   99.99,
			wantGetOk: true,
		},
		{
			name:      "nested map",
			key:       "nested",
			value:     map[string]any{"foo": "bar"},
			getKey:    "nested",
			wantVal:   map[string]any{"foo": "bar"},
			wantGetOk: true,
		},
		{
			name:      "nil value",
			key:       "null",
			value:     nil,
			getKey:    "null",
			wantVal:   nil,
			wantGetOk: true,
		},
		{
			name:      "nonexistent key",
			key:       "exists",
			value:     "value",
			getKey:    "does_not_exist",
			wantVal:   nil,
			wantGetOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := safe_map.New()
			m.Set(tt.key, tt.value)

			got, ok := m.Get(tt.getKey)
			if ok != tt.wantGetOk {
				t.Errorf("Get() ok = %v, want %v", ok, tt.wantGetOk)
			}
			if !reflect.DeepEqual(got, tt.wantVal) {
				t.Errorf("Get() = %v, want %v", got, tt.wantVal)
			}
		})
	}
}

func TestMap_Delete(t *testing.T) {
	tests := []struct {
		name      string
		setup     map[string]any
		deleteKey string
		checkKey  string
		wantExist bool
	}{
		{
			name:      "delete existing key",
			setup:     map[string]any{"a": 1, "b": 2, "c": 3},
			deleteKey: "b",
			checkKey:  "b",
			wantExist: false,
		},
		{
			name:      "delete nonexistent key",
			setup:     map[string]any{"a": 1},
			deleteKey: "b",
			checkKey:  "a",
			wantExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			m.Delete(tt.deleteKey)

			if got := m.Has(tt.checkKey); got != tt.wantExist {
				t.Errorf("Has() = %v, want %v", got, tt.wantExist)
			}
		})
	}
}

func TestMap_Has(t *testing.T) {
	tests := []struct {
		name  string
		setup map[string]any
		key   string
		want  bool
	}{
		{
			name:  "existing key",
			setup: map[string]any{"name": "Alice"},
			key:   "name",
			want:  true,
		},
		{
			name:  "nonexistent key",
			setup: map[string]any{"name": "Alice"},
			key:   "age",
			want:  false,
		},
		{
			name:  "empty map",
			setup: map[string]any{},
			key:   "any",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			if got := m.Has(tt.key); got != tt.want {
				t.Errorf("Has() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMap_Len(t *testing.T) {
	tests := []struct {
		name  string
		setup map[string]any
		want  int
	}{
		{
			name:  "empty map",
			setup: map[string]any{},
			want:  0,
		},
		{
			name:  "one element",
			setup: map[string]any{"a": 1},
			want:  1,
		},
		{
			name:  "multiple elements",
			setup: map[string]any{"a": 1, "b": 2, "c": 3},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			if got := m.Len(); got != tt.want {
				t.Errorf("Len() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMap_Keys(t *testing.T) {
	tests := []struct {
		name  string
		setup map[string]any
		want  []string
	}{
		{
			name:  "empty map",
			setup: map[string]any{},
			want:  []string{},
		},
		{
			name:  "single key",
			setup: map[string]any{"a": 1},
			want:  []string{"a"},
		},
		{
			name:  "sorted keys",
			setup: map[string]any{"c": 3, "a": 1, "b": 2},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "numeric string keys sorted lexicographically",
			setup: map[string]any{"10": 10, "2": 2, "1": 1},
			want:  []string{"1", "10", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			got := m.Keys()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Keys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMap_Values(t *testing.T) {
	tests := []struct {
		name  string
		setup map[string]any
		want  []any
	}{
		{
			name:  "empty map",
			setup: map[string]any{},
			want:  []any{},
		},
		{
			name:  "values in key-sorted order",
			setup: map[string]any{"c": 3, "a": 1, "b": 2},
			want:  []any{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			got := m.Values()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Values() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMap_Clear(t *testing.T) {
	m := FromMap(map[string]any{"a": 1, "b": 2, "c": 3})
	if m.Len() != 3 {
		t.Fatalf("initial Len() = %v, want 3", m.Len())
	}

	m.Clear()

	if m.Len() != 0 {
		t.Errorf("after Clear(), Len() = %v, want 0", m.Len())
	}
	if m.Has("a") {
		t.Errorf("after Clear(), Has(\"a\") = true, want false")
	}
}

func TestMap_Range(t *testing.T) {
	tests := []struct {
		name     string
		setup    map[string]any
		wantKeys []string
		wantVals []any
	}{
		{
			name:     "empty map",
			setup:    map[string]any{},
			wantKeys: []string{},
			wantVals: []any{},
		},
		{
			name:     "sorted iteration",
			setup:    map[string]any{"c": 3, "a": 1, "b": 2},
			wantKeys: []string{"a", "b", "c"},
			wantVals: []any{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			keys := []string{}
			vals := []any{}

			m.Range(func(key string, value any) bool {
				keys = append(keys, key)
				vals = append(vals, value)
				return true
			})

			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("Range() keys = %v, want %v", keys, tt.wantKeys)
			}
			if !reflect.DeepEqual(vals, tt.wantVals) {
				t.Errorf("Range() values = %v, want %v", vals, tt.wantVals)
			}
		})
	}
}

func TestMap_Range_EarlyExit(t *testing.T) {
	m := FromMap(map[string]any{"a": 1, "b": 2, "c": 3})
	var visited []string

	m.Range(func(key string, value any) bool {
		visited = append(visited, key)
		return key != "b" // Stop after "b"
	})

	want := []string{"a", "b"}
	if !reflect.DeepEqual(visited, want) {
		t.Errorf("Range() with early exit visited = %v, want %v", visited, want)
	}
}

func TestMap_Clone(t *testing.T) {
	original := FromMap(map[string]any{"a": 1, "b": 2})
	clone := original.Clone()

	// Verify clone has same content
	if !reflect.DeepEqual(clone.Keys(), original.Keys()) {
		t.Errorf("Clone() keys don't match")
	}

	// Verify modifications to clone don't affect original
	clone.Set("c", 3)
	if original.Has("c") {
		t.Errorf("modifying clone affected original")
	}

	// Verify modifications to original don't affect clone
	original.Set("d", 4)
	if clone.Has("d") {
		t.Errorf("modifying original affected clone")
	}
}

func TestMap_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		setup   map[string]any
		want    string
		wantErr bool
	}{
		{
			name:  "empty map",
			setup: map[string]any{},
			want:  "{}",
		},
		{
			name:  "simple values",
			setup: map[string]any{"name": "Alice", "age": float64(30)},
			want:  `{"age":30,"name":"Alice"}`,
		},
		{
			name:  "nested structure",
			setup: map[string]any{"user": map[string]any{"name": "Bob"}},
			want:  `{"user":{"name":"Bob"}}`,
		},
		{
			name:  "with null",
			setup: map[string]any{"value": nil},
			want:  `{"value":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FromMap(tt.setup)
			got, err := json.Marshal(m)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMap_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "empty object",
			json: "{}",
			want: map[string]any{},
		},
		{
			name: "simple values",
			json: `{"name":"Alice","age":30}`,
			want: map[string]any{"name": "Alice", "age": float64(30)},
		},
		{
			name: "nested structure",
			json: `{"user":{"name":"Bob","active":true}}`,
			want: map[string]any{"user": map[string]any{"name": "Bob", "active": true}},
		},
		{
			name: "with null",
			json: `{"value":null}`,
			want: map[string]any{"value": nil},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := safe_map.New()
			err := json.Unmarshal([]byte(tt.json), m)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := ToMap(m)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("UnmarshalJSON() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMap_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		setup map[string]any
	}{
		{
			name:  "simple values",
			setup: map[string]any{"a": float64(1), "b": "two", "c": true},
		},
		{
			name:  "nested maps",
			setup: map[string]any{"outer": map[string]any{"inner": "value"}},
		},
		{
			name:  "mixed types",
			setup: map[string]any{"str": "hello", "num": float64(42), "bool": false, "null": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := FromMap(tt.setup)

			// Marshal
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal
			decoded := safe_map.New()
			if err := json.Unmarshal(data, decoded); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Compare
			if !reflect.DeepEqual(ToMap(decoded), ToMap(original)) {
				t.Errorf("round trip failed: got %v, want %v", ToMap(decoded), ToMap(original))
			}

			// Verify keys are still sorted
			if !reflect.DeepEqual(decoded.Keys(), original.Keys()) {
				t.Errorf("keys order changed: got %v, want %v", decoded.Keys(), original.Keys())
			}
		})
	}
}

func TestMap_DeterministicOrdering(t *testing.T) {
	// Create multiple maps with same data in different insertion orders
	data := map[string]any{"z": 26, "a": 1, "m": 13, "b": 2, "y": 25}

	m1 := safe_map.New()
	for k, v := range data {
		m1.Set(k, v)
	}

	m2 := FromMap(data)

	// Both should produce the same key order
	keys1 := m1.Keys()
	keys2 := m2.Keys()

	wantKeys := []string{"a", "b", "m", "y", "z"}
	if !reflect.DeepEqual(keys1, wantKeys) {
		t.Errorf("m1.Keys() = %v, want %v", keys1, wantKeys)
	}
	if !reflect.DeepEqual(keys2, wantKeys) {
		t.Errorf("m2.Keys() = %v, want %v", keys2, wantKeys)
	}

	// JSON serialization should be identical
	json1, _ := json.Marshal(m1)
	json2, _ := json.Marshal(m2)

	if string(json1) != string(json2) {
		t.Errorf("JSON serialization differs:\nm1: %s\nm2: %s", json1, json2)
	}
}
