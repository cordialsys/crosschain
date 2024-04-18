package aptos

import (
	"encoding/hex"
	"strings"
)

func mustDecodeHex(h string) []byte {
	h = strings.Replace(h, "0x", "", 1)
	bz, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return bz
}
