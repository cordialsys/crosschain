package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/solana-foundation/solana-go/v2"
)

func init() {
	registry.RegisterTxVariantInput(&CallInput{})
}

type CallInput struct {
	TxInput
}

// SetFeeConfig preserves the absolute inline budget of a prebuilt transaction.
func (input *CallInput) SetFeeConfig(solTx *solana.Transaction) {
	input.TransactionConfig = nil
	input.SignatureCount = solTx.Message.Header.NumRequiredSignatures
	switch solTx.Message.GetVersion() {
	case solana.MessageVersionV1:
		config := solTx.Message.TransactionConfig
		input.TransactionConfig = &config
		input.TransactionVersion = TransactionVersionV1
	case solana.MessageVersionV0:
		input.TransactionVersion = TransactionVersionV0
	default:
		input.TransactionVersion = TransactionVersionLegacy
	}
}

func (input *CallInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	if input.TransactionConfig == nil {
		return input.TxInput.GetFeeLimit()
	}
	fee := input.V1PriorityFee()
	// FetchBaseInput may reserve rent for a nonce account that this prebuilt
	// call never creates. Only its actual signatures contribute to the base fee.
	baseFee := xc.NewAmountBlockchainFromUint64(uint64(input.SignatureCount) * LamportsPerSignature)
	return fee.Add(&baseFee), ""
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
	input.SetFeeConfig(solanaTx)
	// We want to sync the information from the call (which may have been edited via .SetInput()).
	// This allows the best chance conflict resolution resolution to work.

	// Take whatever recent-blockhash that it ended up using.
	input.TxInput.RecentBlockHash = solanaTx.Message.RecentBlockhash
	usingDurableNonce, _, _ := input.DoesTxUseOurDurableNonce(solanaTx)
	if usingDurableNonce {
		// If the transaction is using a durable nonce, then the recent-blockhash is the durable nonce,
		// and we should sync that too for conflict resolution.
		input.TxInput.DurableNonce = solanaTx.Message.RecentBlockhash
	} else {
		input.TxInput.DurableNonceAccount = solana.PublicKey{}
		input.TxInput.DurableNonce = solana.Hash{}
	}
	return nil
}
