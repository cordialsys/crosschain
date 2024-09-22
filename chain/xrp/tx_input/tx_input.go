package tx_input

import (
	"encoding/base64"
	"encoding/hex"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	Sequence           int `json:"Sequence"`
	LastLedgerSequence int `json:"LastLedgerSequence"`
	PublicKey          []byte
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
	// multiply the gas price using the default, or apply a strategy according to the enum
	_ = multiplier
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// are these two transactions independent (e.g. different sequences & utxos & expirations?)
	if emvOther, ok := other.(*TxInput); ok {
		return emvOther.Sequence != input.Sequence
	}

	return true
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
