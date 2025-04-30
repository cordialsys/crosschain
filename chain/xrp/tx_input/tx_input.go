package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope

	// Included for treasury versions <= 25.6.x
	XSequence           int64 `json:"Sequence"`
	XLastLedgerSequence int64 `json:"LastLedgerSequence"`

	// Renamed the fields to use snake_case for consistency
	V2Sequence           int64 `json:"sequence"`
	V2LastLedgerSequence int64 `json:"last_ledger_sequence"`

	Fee              xc.AmountBlockchain `json:"fee,omitempty"`
	AccountDeleteFee xc.AmountBlockchain `json:"delete_account_fee,omitempty"`
	ReserveAmount    xc.AmountBlockchain `json:"reserve_amount,omitempty"`
	XrpBalance       xc.AmountBlockchain `json:"xrp_balance,omitempty"`
	// Indicate if account-delete is needed to send the full amount requested
	AccountDelete bool `json:"account_delete,omitempty"`
}

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverXrp,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverXrp
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}

	product := multiplier.Mul(decimal.NewFromBigInt(input.Fee.Int(), 0)).BigInt()
	input.Fee = xc.AmountBlockchain(*product)
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	if input.AccountDelete {
		return input.ReserveAmount, ""
	}
	return input.Fee, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// are these two transactions independent (e.g. different sequences & utxos & expirations?)
	if emvOther, ok := other.(*TxInput); ok {
		return emvOther.V2Sequence != input.V2Sequence
	}

	return false
}

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	// safe from double send ?
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}

	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}

	return true
}
