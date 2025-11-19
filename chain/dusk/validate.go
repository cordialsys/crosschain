package dusk

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/cloudflare/circl/sign/bls"
	xc "github.com/cordialsys/crosschain"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	// Decode base58
	decoded := base58.Decode(addrStr)
	if len(decoded) == 0 {
		return fmt.Errorf("invalid dusk address %s: invalid base58 encoding", address)
	}

	// Dusk addresses are BLS G2 public keys, which should be 96 bytes
	if len(decoded) != 96 {
		return fmt.Errorf("invalid dusk address %s: invalid length %d, expected 96", address, len(decoded))
	}

	// Try to unmarshal as BLS G2 public key
	var publicKey bls.PublicKey[bls.G2]
	err := publicKey.UnmarshalBinary(decoded)
	if err != nil {
		return fmt.Errorf("invalid dusk address %s: not a valid BLS public key: %w", address, err)
	}

	return nil
}
