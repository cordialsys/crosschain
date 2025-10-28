package tx_input

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const FeeMargin = 500

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Utxos                   []types.Utxo             `json:"utxos"`
	Slot                    uint64                   `json:"slot"`
	Fee                     uint64                   `json:"fee"`
	TransactionValidityTime uint64                   `json:"transaction_validity_time"`
	ProtocolParams          types.ProtocolParameters `json:"protocol_params"`
}

type UtxoGetter interface {
	GetUtxos() []types.Utxo
}

var _ xc.TxInput = &TxInput{}
var _ UtxoGetter = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverCardano,
		},
	}
}

func (input *TxInput) GetUtxos() []types.Utxo {
	return input.Utxos
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCardano
}

// Cardano does not support fee bidding
func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(input.Fee), ""
}

// check if any utxo is spent twice
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	cardanoOther, ok := other.(UtxoGetter)
	if !ok {
		return
	}

	for _, utxo1 := range input.GetUtxos() {
		for _, utxo2 := range cardanoOther.GetUtxos() {
			if utxo1.TxHash == utxo2.TxHash && utxo1.Index == utxo2.Index {
				// not independent
				return false
			}
		}
	}

	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	// check if of the same types
	if _, ok := other.(UtxoGetter); !ok {
		return false
	}

	// we are risking double send if we have any independent utxo's
	if input.IndependentOf(other) {
		return false
	}
	// conflicting utxo for all - we're safe
	return true
}

func (input *TxInput) CalculateTxFee(tx xc.Tx) error {
	cbor, err := tx.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize tx: %w", err)
	}

	txSize := len(cbor)
	input.Fee = input.ProtocolParams.FeePerByte*uint64(txSize) + input.ProtocolParams.FixedFee + FeeMargin
	return nil
}

type StakingInput struct {
	TxInput
	KeyDeposit uint64
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*StakingInput) Staking() {}
func (*StakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverCardano, string(xc.Native))
}

type UnstakingInput struct {
	TxInput
	KeyDeposit uint64
}

var _ xc.TxVariantInput = &UnstakingInput{}
var _ xc.UnstakeTxInput = &UnstakingInput{}

func (*UnstakingInput) Unstaking() {}
func (*UnstakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverCardano, string(xc.Native))
}

type WithdrawInput struct {
	TxInput
	RewardsAddress xc.Address
	RewardsAmount  xc.AmountBlockchain
}

var _ xc.TxVariantInput = &WithdrawInput{}
var _ xc.WithdrawTxInput = &WithdrawInput{}

func (*WithdrawInput) Withdrawing() {}
func (*WithdrawInput) GetVariant() xc.TxVariantInputType {
	return xc.NewWithdrawingInputType(xc.DriverCardano, string(xc.Native))
}
