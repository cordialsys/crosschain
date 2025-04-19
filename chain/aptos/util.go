package aptos

import (
	"encoding/hex"
	"fmt"
	"strings"

	transactionbuilder "github.com/coming-chat/go-aptos/transaction_builder"
)

func DecodeHex(h string) ([]byte, error) {
	h = strings.Replace(h, "0x", "", 1)
	bz, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func DecodeAddress(h string) ([transactionbuilder.ADDRESS_LENGTH]byte, error) {
	h = strings.Replace(h, "0x", "", 1)
	if len(h) < 64 && len(h) > 32 {
		// zero-pad
		h = strings.Repeat("0", 64-len(h)) + h
	}
	decoded, err := DecodeHex(h)
	if err != nil {
		return [transactionbuilder.ADDRESS_LENGTH]byte{}, fmt.Errorf("failed to decode address '%s': %w", h, err)
	}
	if len(decoded) != transactionbuilder.ADDRESS_LENGTH {
		return [transactionbuilder.ADDRESS_LENGTH]byte{}, fmt.Errorf("invalid address length for '%s': %d", h, len(decoded))
	}
	return [transactionbuilder.ADDRESS_LENGTH]byte(decoded), nil
}
