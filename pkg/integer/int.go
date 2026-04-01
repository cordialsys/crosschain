package integer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Uint64 uint64
type Int64 int64

// Unmarshal an integer, treating it literally as nanoseconds if it does not already have a unit
type Duration time.Duration

// MarshalJSON outputs as a JSON string to avoid float64 precision loss
// when roundtripping through map[string]interface{}.
func (b Uint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(b), 10))
}

func (b *Uint64) UnmarshalJSON(data []byte) error {
	// Parse directly from the raw JSON bytes to avoid float64 precision loss
	// that occurs when unmarshaling large integers via interface{}.
	s := strings.TrimSpace(string(data))
	// Strip quotes if it's a JSON string
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "null" || s == "" {
		*b = 0
		return nil
	}
	// Truncate any decimal portion
	s = strings.Split(s, ".")[0]
	result, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse value as int: %v", err)
	}
	*b = Uint64(result)
	return nil
}

// MarshalJSON outputs as a JSON string to avoid float64 precision loss
// when roundtripping through map[string]interface{}.
func (b Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(b), 10))
}

func (b *Int64) UnmarshalJSON(data []byte) error {
	// Parse directly from the raw JSON bytes to avoid float64 precision loss
	// that occurs when unmarshaling large integers via interface{}.
	s := strings.TrimSpace(string(data))
	// Strip quotes if it's a JSON string
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "null" || s == "" {
		*b = 0
		return nil
	}
	// Truncate any decimal portion
	s = strings.Split(s, ".")[0]
	result, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("could not parse value as int: %v", err)
	}
	*b = Int64(result)
	return nil
}

func (b *Duration) UnmarshalJSON(data []byte) error {
	var v interface{}
	var err error

	if err = json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid json: %v", err)
	}

	// try parsing as a duration string
	if vString, ok := v.(string); ok {
		parsed, err := time.ParseDuration(vString)
		if err == nil {
			// ok
			*b = Duration(parsed)
			return nil
		}
		// try with default unit
		withUnit := string(data) + "ns"
		parsed, err = time.ParseDuration(withUnit)
		if err == nil {
			// ok
			*b = Duration(parsed)
			return nil
		}
	}

	// try parsing as integer
	result, err := ParseInt64Fuzzy(v)
	if err != nil {
		return fmt.Errorf("invalid integer: %v", err)
	}
	*b = Duration(result)
	return nil
}

// Go doesn't define marshal methods for time.Duration, meaning it can produce
// undeterministic results ("an unfortunate oversight" https://github.com/golang/go/issues/10275)
// So we need to define it.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func ParseInt64Fuzzy(v interface{}) (int64, error) {
	s := ""

	switch v := v.(type) {
	case string:
		s = v
	case float32:
		s = fmt.Sprintf("%d", int64(v))
	case float64:
		s = fmt.Sprintf("%d", int64(v))
	default:
		s = fmt.Sprintf("%d", v)
	}

	s = strings.Split(s, ".")[0]
	f, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func ParseUInt64Fuzzy(v interface{}) (uint64, error) {
	s := ""

	switch v := v.(type) {
	case string:
		s = v
	case float32:
		s = fmt.Sprintf("%d", uint64(v))
	case float64:
		s = fmt.Sprintf("%d", uint64(v))
	default:
		s = fmt.Sprintf("%d", v)
	}

	s = strings.Split(s, ".")[0]
	f, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint64(f), nil
}
