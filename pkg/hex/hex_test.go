package hex

import (
	"encoding/json"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

type object struct {
	Hex Hex `toml:"hex"`
}

func TestHex(t *testing.T) {
	var err error
	hex := Hex([]byte{01, 02, 03, 04})
	require.Equal(t, "01020304", hex.String())
	require.Equal(t, []byte{01, 02, 03, 04}, hex.Bytes())

	hex2 := Hex{}
	err = json.Unmarshal([]byte(`"01020304"`), &hex2)
	require.NoError(t, err)
	require.Equal(t, []byte{01, 02, 03, 04}, hex2.Bytes())

	hex3 := Hex{}
	err = json.Unmarshal([]byte(`"0x01020304"`), &hex3)
	require.NoError(t, err)
	require.Equal(t, []byte{01, 02, 03, 04}, hex3.Bytes())

	// test toml
	hex4 := object{Hex: Hex{}}
	err = toml.Unmarshal([]byte(`hex = "01020304"`), &hex4)
	require.NoError(t, err)
	require.Equal(t, []byte{01, 02, 03, 04}, hex4.Hex.Bytes())

	hex5 := object{Hex: Hex{}}
	err = toml.Unmarshal([]byte(`hex = '01020304'`), &hex5)
	require.NoError(t, err)
	require.Equal(t, []byte{01, 02, 03, 04}, hex5.Hex.Bytes())

	bz, err := toml.Marshal(object{Hex: hex4.Hex})
	require.NoError(t, err)
	require.Equal(t, "hex = '01020304'\n", string(bz))
}
