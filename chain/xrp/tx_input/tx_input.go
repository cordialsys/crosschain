package tx_input

import (
	"encoding/base64"
	"encoding/hex"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	XRPTx          tx.XRPTransaction // TODO: Remove XRPTx and add fields for only things taken from ledger.
	SerializeXRPTx []byte
	Pubkey         []byte
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

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}

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
	// multiply the gas price using the default, or apply a strategy according to the enum
	_ = multiplier
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// are these two transactions independent (e.g. different sequences & utxos & expirations?)
	return true
}

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	// safe from double send ?
	return true
}
