package common

import (
	"math"

	xc "github.com/cordialsys/crosschain"
)

func VotesToTrx(v uint64, decimals int) xc.AmountBlockchain {
	d := math.Pow10(decimals)
	trxRaw := uint64(float64(v) * d)
	return xc.NewAmountBlockchainFromUint64(trxRaw)
}

// Votes are always a positive natural numbers, we have to truncate the TRX amount.
func TrxToVotes(trx xc.AmountBlockchain, decimals int) uint64 {
	d := math.Pow10(decimals)
	divisor := xc.NewAmountBlockchainFromUint64(uint64(d))
	return trx.Div(&divisor).Uint64()
}
