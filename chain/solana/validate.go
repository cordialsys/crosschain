package solana

import (
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
)

func ValidateAddress(cfg *xc.ChainBaseConfig, address xc.Address) error {
	addrStr := string(address)

	decoded := base58.Decode(addrStr)
	if len(decoded) == 0 {
		return fmt.Errorf("invalid solana address %s: invalid base58 encoding", address)
	}

	// Solana addresses are Ed25519 public keys (32 bytes)
	if len(decoded) != 32 {
		return fmt.Errorf("invalid solana address %s: must be 32 bytes (got %d)", address, len(decoded))
	}

	return nil
}
