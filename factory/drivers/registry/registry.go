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
	_, ok1 := variant.(xc.StakeTxInput)
	_, ok2 := variant.(xc.UnstakeTxInput)
	_, ok3 := variant.(xc.WithdrawTxInput)
	_, ok4 := variant.(xc.MultiTransferInput)
	_, ok5 := variant.(xc.CallTxInput)
	if !ok1 && !ok2 && !ok3 && !ok4 && !ok5 {
		panic(fmt.Sprintf("staking input %T must implement one of known variants", variant))
	}

	supportedVariantTx = append(supportedVariantTx, variant)
}

func GetSupportedTxVariants() []xc.TxVariantInput {
	return supportedVariantTx
}
