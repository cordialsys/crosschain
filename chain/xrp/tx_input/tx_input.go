package tx_input

import (
	"encoding/base64"
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Sequence           int64               `json:"Sequence"`
	LastLedgerSequence int64               `json:"LastLedgerSequence"`
	PublicKey          []byte              `json:"public_key,omitempty"`
	Fee                xc.AmountBlockchain `json:"fee,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func (input *TxInput) SetPublicKey(publicKey []byte) error {
	input.PublicKey = publicKey
	return nil
}

func (input *TxInput) GetPublicKey() []byte {
	return input.PublicKey
}

func (input *TxInput) SetPublicKeyFromStr(publicKeyStr string) error {
	var (
		publicKey []byte
		err       error
	)

	if len(publicKeyStr) == 128 || len(publicKeyStr) == 130 {
		publicKey, err = hex.DecodeString(publicKeyStr)
		if err != nil {
			return err
		}
	} else {
		publicKey, err = base64.RawStdEncoding.DecodeString(publicKeyStr)
		if err != nil {
			return err
		}
	}

	input.PublicKey = publicKey

	return nil
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

func (input *TxInput) GetMaxFee() (xc.AmountBlockchain, xc.ContractAddress) {
	return input.Fee, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// are these two transactions independent (e.g. different sequences & utxos & expirations?)
	if emvOther, ok := other.(*TxInput); ok {
		return emvOther.Sequence != input.Sequence
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
