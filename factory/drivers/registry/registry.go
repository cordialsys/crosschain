package registry

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

var supportedBaseInputTx = []xc.TxInput{}
var supportedVariantTx = []xc.TxVariantInput{}

func RegisterTxBaseInput(txInput xc.TxInput) {
	for _, existing := range supportedBaseInputTx {
		if existing.GetDriver() == txInput.GetDriver() {
			panic(fmt.Sprintf("base input %T driver %s duplicates %T", txInput, txInput.GetDriver(), existing))
		}
	}
	supportedBaseInputTx = append(supportedBaseInputTx, txInput)
}

func GetSupportedBaseTxInputs() []xc.TxInput {
	return supportedBaseInputTx
}
func RegisterTxVariantInput(variant xc.TxVariantInput) {
	for _, existing := range supportedVariantTx {
		if existing.GetVariant() == variant.GetVariant() {
			panic(fmt.Sprintf("staking input %T driver %s duplicates %T", variant, variant.GetVariant(), existing))
		}
	}
	i1, ok1 := variant.(xc.StakeTxInput)
	i2, ok2 := variant.(xc.UnstakeTxInput)
	i3, ok3 := variant.(xc.WithdrawTxInput)
	i4, ok4 := variant.(xc.MultiTransferInput)
	if !ok1 && !ok2 && !ok3 && !ok4 {
		panic(fmt.Sprintf("staking input %T must implement one of %T, %T, %T, %T", variant, i1, i2, i3, i4))
	}

	supportedVariantTx = append(supportedVariantTx, variant)
}

func GetSupportedTxVariants() []xc.TxVariantInput {
	return supportedVariantTx
}
