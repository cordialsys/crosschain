package tx_input

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for Substrate
type TxInput struct {
	xc.TxInputEnvelope
	Meta          Metadata             `json:"meta,omitempty"`
	GenesisHash   types.Hash           `json:"genesis_hash,omitempty"`
	CurHash       types.Hash           `json:"current_hash,omitempty"`
	Rv            types.RuntimeVersion `json:"runtime_version,omitempty"`
	CurrentHeight uint64               `json:"current_height,omitempty"`
	Tip           uint64               `json:"tip,omitempty"`
	Nonce         uint64               `json:"account_nonce,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.StakeTxInput = &TxInput{}
var _ xc.UnstakeTxInput = &TxInput{}
var _ xc.WithdrawTxInput = &TxInput{}

// NewTxInput returns a new Substrate TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSubstrate),
	}
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&TxInput{})
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverSubstrate
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedTip := multiplier.Mul(decimal.NewFromInt(int64(input.Tip)))
	input.Tip = multipliedTip.BigInt().Uint64()
	return nil
}

func (input *TxInput) GetMaxFee() (xc.AmountBlockchain, xc.ContractAddress) {
	// very simple, just tip!
	maxSpend := xc.NewAmountBlockchainFromUint64(input.Tip)
	return maxSpend, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if substrateOther, ok := other.(*TxInput); ok {
		return substrateOther.Nonce != input.Nonce
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	// all same sequence means no double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// sequence all same - we're safe
	return true
}

func (input *TxInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverSubstrate, string(xc.Native))
}
func (input *TxInput) Staking()     {}
func (input *TxInput) Unstaking()   {}
func (input *TxInput) Withdrawing() {}
