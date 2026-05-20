package evm

import (
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

func ValidateConfig(chain *xc.ChainConfig) error {
	// Mainnet EVM chains must have a valid integer chain_id set
	if chain.Network == "mainnet" {
		if chain.ChainID == "" {
			return fmt.Errorf("%s: mainnet evm chain must have chain_id set", chain.Chain)
		}
		if _, ok := chain.ChainID.AsInt(); !ok {
			return fmt.Errorf("%s: chain_id must be a valid integer, got %q", chain.Chain, chain.ChainID)
		}
	}
	if chain.Staking.Enabled() {
		// must have a batch-deposit contract set
		contract := chain.Staking.StakeContract
		bz, err := hex.DecodeString(strings.TrimPrefix(contract, "0x"))
		if err != nil {
			return err
		}
		if len(bz) == 0 {
			return fmt.Errorf("must set staking.stake_contract")
		}

		// must have a request-exit contract set
		contract = chain.Staking.UnstakeContract
		bz, err = hex.DecodeString(strings.TrimPrefix(contract, "0x"))
		if err != nil {
			return err
		}
		if len(bz) == 0 {
			return fmt.Errorf("must set staking.unstake_contract")
		}
	}
	return nil
}
