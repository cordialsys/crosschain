package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

func init() {
	registry.RegisterTxVariantInput(&CallInput{})
}

type CallInput struct {
	TxInput
}

var _ xc.TxVariantInput = &CallInput{}
var _ xc.CallTxInput = &CallInput{}

func (*CallInput) Calling() {}

func (*CallInput) GetVariant() xc.TxVariantInputType {
	return xc.NewCallingInputType(xc.DriverSolana)
}
