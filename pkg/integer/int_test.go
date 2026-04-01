package integer_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cordialsys/crosschain/pkg/integer"
	"github.com/stretchr/testify/require"
)

type TestFuzzyIntStruct struct {
	A integer.Uint64 `json:"a"`
	B *integer.Int64 `json:"b"`
}

func TestFuzzyInt(t *testing.T) {
	val := TestFuzzyIntStruct{}

	err := json.Unmarshal([]byte(`{"a": 123}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123, val.A)

	err = json.Unmarshal([]byte(`{"a": "123"}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123, val.A)

	err = json.Unmarshal([]byte(`{"a": "123.5"}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123, val.A)

	err = json.Unmarshal([]byte(`{"a": "123.1243435345"}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123, val.A)

	err = json.Unmarshal([]byte(`{"b": "123.1243435345"}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123, *val.B)

	err = json.Unmarshal([]byte(`{"b": 123000000000}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 123000000000, *val.B)

	err = json.Unmarshal([]byte(`{"b": "ABC"}`), &val)
	require.Error(t, err)

	b := integer.Int64(456)
	ref := TestFuzzyIntStruct{
		A: integer.Uint64(123),
		B: &b,
	}
	bz, err := json.Marshal(ref)
	require.NoError(t, err)

	require.Equal(t, `{"a":"123","b":"456"}`, string(bz))

	err = json.Unmarshal([]byte(`{"a": 3600000000000}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 3600000000000, val.A)

	// Test large int64 values that exceed float64 precision (>2^53)
	err = json.Unmarshal([]byte(`{"b": 265892813885931521}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 265892813885931521, *val.B)

	// Same value as a string
	err = json.Unmarshal([]byte(`{"b": "265892813885931521"}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 265892813885931521, *val.B)

	// Large uint64
	err = json.Unmarshal([]byte(`{"a": 265892813885931521}`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 265892813885931521, val.A)
}

type TestDurationStruct struct {
	A integer.Duration `json:"a"`
}

func TestDurationLiteral(t *testing.T) {
	val := integer.Duration(0)
	err := json.Unmarshal([]byte(`"1234"`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 1234, time.Duration(val).Nanoseconds())

	val = integer.Duration(0)
	err = json.Unmarshal([]byte(`"1234h"`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 1234, time.Duration(val).Hours())

	val = integer.Duration(0)
	err = json.Unmarshal([]byte(`"0s"`), &val)
	require.NoError(t, err)
	require.EqualValues(t, 0, time.Duration(val).Hours())
	require.EqualValues(t, 0, time.Duration(val).Seconds())

	val = integer.Duration(time.Hour * 1234)
	bz, err := val.MarshalJSON()
	require.NoError(t, err)
	require.EqualValues(t, string(`"1234h0m0s"`), string(bz))

	asStruct := &TestDurationStruct{}
	err = json.Unmarshal([]byte(`{"a": "1234s"}`), &asStruct)
	require.NoError(t, err)
	require.EqualValues(t, 1234, time.Duration(asStruct.A).Seconds())

	bz, err = json.Marshal(asStruct)
	require.NoError(t, err)
	asStruct = &TestDurationStruct{}
	err = json.Unmarshal(bz, &asStruct)
	require.NoError(t, err)
}
