package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/gagliardetto/solana-go"
)

func init() {
	registry.RegisterTxVariantInput(&CallInput{})
}

type CallInput struct {
	TxInput
}

var _ xc.TxInput = &CallInput{}
var _ xc.TxInputWithCall = &CallInput{}
var _ xc.TxVariantInput = &CallInput{}
var _ xc.CallTxInput = &CallInput{}

func (*CallInput) Calling() {}

func (*CallInput) GetVariant() xc.TxVariantInputType {
	return xc.NewCallingInputType(xc.DriverSolana)
}

func NewCallPayload(solTx *solana.Transaction) *CallPayload {
	return &CallPayload{solTx}
}

type CallPayload struct {
	solTx *solana.Transaction
}

var _ xc.TxCallPayload = &CallPayload{}

func (p *CallPayload) IsTxCallPayload() {}

func (input *CallInput) SetCall(call xc.TxCallPayload) error {
	txCall, ok := call.(*CallPayload)
	if !ok {
		return fmt.Errorf("invalid call payload for solana: %T", call)
	}

	solanaTx := txCall.solTx
	// We want to sync the information from the call (which may have been edited via .SetInput()).
	// This allows the best chance conflict resolution resolution to work.

	// Take whatever recent-blockhash that it ended up using.
	input.TxInput.RecentBlockHash = solanaTx.Message.RecentBlockhash
	if input.DoesTxUseOurDurableNonce(solanaTx) {
		// If the transaction is using a durable nonce, then the recent-blockhash is the durable nonce,
		// and we should sync that too for conflict resolution.
		input.TxInput.DurableNonce = solanaTx.Message.RecentBlockhash
	} else {
		input.TxInput.DurableNonceAccount = solana.PublicKey{}
		input.TxInput.DurableNonce = solana.Hash{}
	}
	return nil
}
