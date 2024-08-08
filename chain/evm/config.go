package evm

import (
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

func ValidateConfig(chain *xc.ChainConfig) error {
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
