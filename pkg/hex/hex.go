package hex

import (
	"encoding/hex"
	"encoding/json"
	"strings"
)

var EncodeToString = hex.EncodeToString
var DecodeString = hex.DecodeString

type Hex []byte

func (h Hex) String() string {
	return hex.EncodeToString(h)
}

func (h Hex) Bytes() []byte {
	return []byte(h)
}

func (h Hex) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

func decodeHex(bz []byte) (Hex, error) {
	// drop quotes
	var s string
	s = strings.Trim(string(bz), "\"")
	s = strings.Trim(s, "'")
	s = strings.TrimPrefix(s, "0x")
	bz, err := hex.DecodeString(s)
	if err != nil {
		return bz, err
	}
	return bz, nil
}

func (h *Hex) UnmarshalJSON(data []byte) error {
	bz, err := decodeHex(data)
	if err != nil {
		return err
	}
	*h = bz
	return nil
}

func (h *Hex) UnmarshalText(data []byte) error {
	bz, err := decodeHex(data)
	if err != nil {
		return err
	}
	*h = bz
	return nil
}

func (h Hex) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}
