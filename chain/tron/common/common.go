package common

import (
	"errors"

	xc "github.com/cordialsys/crosschain"
)

func VotesToTrx(v uint64, decimals int) xc.AmountBlockchain {
	trx := xc.NewAmountHumanReadableFromBlockchain(xc.NewAmountBlockchainFromUint64(v))
	return trx.ToBlockchain(int32(decimals))
}

// Votes are always a positive natural numbers, we have to truncate the TRX amount.
func TrxToVotes(trx xc.AmountBlockchain, decimals int) (uint64, error) {
	hmr := trx.ToHuman(int32(decimals))
	if hmr.Decimal().IntPart() < 0 {
		return 0, errors.New("votes are expected to be positive")
	}
	return hmr.Decimal().BigInt().Uint64(), nil
}
