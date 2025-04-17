package aptos

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func DecodeHex(h string) ([]byte, error) {
	h = strings.Replace(h, "0x", "", 1)
	bz, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("failed to decode address: %s: %w", h, err)
	}
	return bz, nil
}
