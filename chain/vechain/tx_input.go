package vechain

import (
	xc "github.com/cordialsys/crosschain"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// VTHOContract is the native energy token used to pay Thor transaction fees.
const VTHOContract xc.ContractAddress = "0x0000000000000000000000000000456e65726779"

type TxInput struct {
	// Reuse the EVM gas_limit field for the complete client-estimated gas budget.
	// Callers may override it before building; the builder uses it unchanged.
	evminput.TxInput
	ChainTag   byte   `json:"chain_tag"`
	BlockRef   uint64 `json:"block_ref"`
	Expiration uint32 `json:"expiration"`
}

func init() { registry.RegisterTxBaseInput(&TxInput{}) }

func NewTxInput() *TxInput {
	input := &TxInput{TxInput: *evminput.NewTxInput(), Expiration: 720}
	input.Type = xc.DriverVeChain
	return input
}

func (*TxInput) GetDriver() xc.Driver { return xc.DriverVeChain }
func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gas := xc.NewAmountBlockchainFromUint64(input.GasLimit)
	return input.GasFeeCap.Mul(&gas), VTHOContract
}
func (*TxInput) IsFeeLimitAccurate() bool { return false }

// Thor nonces are random, not account sequence numbers. The block reference
// also participates in transaction identity, so reusing just a nonce is unsafe.
func (input *TxInput) IndependentOf(other xc.TxInput) bool {
	previous, ok := other.(*TxInput)
	return ok && previous != nil && !input.SafeFromDoubleSend(previous)
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) bool {
	previous, ok := other.(*TxInput)
	// Only an identical signing envelope guarantees an identical transaction ID.
	// Fee changes produce a new ID even when the nonce and block reference match.
	return ok && previous != nil && input.Nonce == previous.Nonce && input.BlockRef == previous.BlockRef &&
		input.ChainTag == previous.ChainTag && input.Expiration == previous.Expiration &&
		input.GasLimit == previous.GasLimit && input.GasFeeCap.Cmp(&previous.GasFeeCap) == 0 && input.GasTipCap.Cmp(&previous.GasTipCap) == 0
}
