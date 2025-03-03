package tx_input

import (
	"encoding/base64"
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

type TxInput struct {
	xc.TxInputEnvelope
	SequenceNumber uint64 `json:"sequence_number,omitempty"`
	GasLimit       uint64 `json:"gas_limit,omitempty"`
	GasPrice       uint64 `json:"gas_price,omitempty"`
	Timestamp      uint64 `json:"timestamp,omitempty"`
	ChainId        int    `json:"chain_id,omitempty"`
	Pubkey         []byte `json:"pubkey,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverAptos,
		},
	}
}
func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverAptos
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	input.GasPrice = multiplier.Mul(decimal.NewFromInt(int64(input.GasPrice))).BigInt().Uint64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	gasLimit := xc.NewAmountBlockchainFromUint64(input.GasLimit)
	gasPrice := xc.NewAmountBlockchainFromUint64(input.GasPrice)
	maxFeeSpend := gasLimit.Mul(&gasPrice)
	return maxFeeSpend, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if aptosOther, ok := other.(*TxInput); ok {
		return aptosOther.SequenceNumber != input.SequenceNumber
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

func (input *TxInput) SetPublicKey(pubkey []byte) error {
	input.Pubkey = pubkey
	return nil
}

func (input *TxInput) SetPublicKeyFromStr(pubkeyStr string) error {
	var err error
	var pubkey []byte
	if len(pubkeyStr) == 128 || len(pubkeyStr) == 130 {
		pubkey, err = hex.DecodeString(pubkeyStr)
		if err != nil {
			return err
		}
	} else {
		pubkey, err = base64.RawStdEncoding.DecodeString(pubkeyStr)
		if err != nil {
			return err
		}
	}
	input.Pubkey = pubkey
	return nil
}
