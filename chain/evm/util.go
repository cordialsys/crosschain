package evm

import (
	"math/big"

	xc "github.com/cordialsys/crosschain"
)

func GweiToWei(gwei uint64) xc.AmountBlockchain {
	bigGwei := big.NewInt(int64(gwei))

	ten := big.NewInt(10)
	nine := big.NewInt(9)
	factor := big.NewInt(0).Exp(ten, nine, nil)

	bigGwei.Mul(bigGwei, factor)
	return xc.AmountBlockchain(*bigGwei)
}
