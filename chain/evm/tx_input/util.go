package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/shopspring/decimal"
)

func Count32EthChunks(chain *xc.ChainConfig, amount xc.AmountBlockchain) (uint64, error) {
	ethInc, _ := xc.NewAmountHumanReadableFromStr("32")
	decimals := int32(18)
	if chain.Decimals != 0 {
		decimals = chain.Decimals
	}
	weiInc := ethInc.ToBlockchain(decimals)

	if amount.Cmp(&weiInc) < 0 {
		return 0, fmt.Errorf("must stake at least 32 ether")
	}
	amountHuman := amount.ToHuman(decimals)

	quot := amountHuman.Div(ethInc)
	rounded := (decimal.Decimal)(quot).Round(0)
	if quot.String() != rounded.String() {
		return 0, fmt.Errorf("must stake an increment of 32 ether")
	}
	return quot.ToBlockchain(0).Uint64(), nil
}
