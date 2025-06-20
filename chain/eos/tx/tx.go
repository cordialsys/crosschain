package tx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	ecc2 "github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
)

// Tx for Template
type Tx struct {
	chain      *xc.ChainBaseConfig
	input      *tx_input.TxInput
	signatures []xc.TxSignature
	builtTx    *eos.Transaction
}

var _ xc.Tx = &Tx{}

// It's necessary to have to keep requesting signatures repeatedly until one
// of EOS's 'canoncical' signatures are found.
var _ xc.TxAdditionalSighashes = &Tx{}

func NewTx(chain *xc.ChainBaseConfig, input *tx_input.TxInput, builtTx *eos.Transaction) *Tx {
	return &Tx{chain, input, []xc.TxSignature{}, builtTx}
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	eosTx, err := tx.BuildTx()
	if err != nil {
		return ""
	}
	lastSig, ok := tx.LastSignature()
	if !ok {
		return ""
	}
	packedTrx, err := tx.SignAndPack(eosTx, lastSig)
	if err != nil {
		return ""
	}
	trxID, err := packedTrx.ID()
	if err != nil {
		return ""
	}
	return xc.TxHash(trxID.String())
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	eosTx, err := tx.BuildTx()
	if err != nil {
		return nil, err
	}
	sigDigest, err := Sighash(eosTx, tx.input.ChainID)
	if err != nil {
		return nil, err
	}

	return []*xc.SignatureRequest{
		xc.NewSignatureRequest(sigDigest),
	}, nil
}

func (tx Tx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	lastSig, ok := tx.LastSignature()
	if ok {
		if IsCanonical(lastSig) {
			// the search is over :)
			return nil, nil
		}
	}

	// Have to keep trying...
	if len(tx.signatures) > 255 {
		return nil, errors.New("could not find canonical EOS signature")
	}
	eosTx, err := tx.BuildTx()
	if err != nil {
		return nil, err
	}

	sigDigest, err := Sighash(eosTx, tx.input.ChainID)
	if err != nil {
		return nil, err
	}

	return []*xc.SignatureRequest{
		xc.NewSignatureRequest(sigDigest),
	}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	tx.signatures = []xc.TxSignature{}
	for _, sig := range sigs {
		canonicalSigMaybe := SwapRecoveryByte(sig.Signature)
		tx.signatures = append(tx.signatures, canonicalSigMaybe)
	}
	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.signatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	eosTx, err := tx.BuildTx()
	if err != nil {
		return nil, err
	}
	lastSig, ok := tx.LastSignature()
	if !ok {
		return nil, errors.New("EOS tx not signed")
	}

	packedTrx, err := tx.SignAndPack(eosTx, lastSig)
	if err != nil {
		return nil, err
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)

	err = encoder.Encode(packedTrx)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
func jsonPrint(v interface{}) {
	json, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(json))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) BuildTx() (*eos.Transaction, error) {
	eosTx := *tx.builtTx
	// The signer may be using deterministic signatures, so we need to make
	// some useless change in the signature body to force a completely different signature.
	eosTx.MaxCPUUsageMS = byte(tx.SigCount())
	return &eosTx, nil
}

func (tx Tx) SignAndPack(eosTx *eos.Transaction, signature xc.TxSignature) (*eos.PackedTransaction, error) {
	signedTx := eos.NewSignedTransaction(eosTx)
	withPrefix := append([]byte{byte(ecc2.CurveK1)}, signature...)
	sigFormatted, err := ecc2.NewSignatureFromData(withPrefix)
	if err != nil {
		return nil, err
	}
	signedTx.Signatures = []ecc2.Signature{sigFormatted}
	packedTrx, err := signedTx.Pack(eos.CompressionNone)
	if err != nil {
		return nil, err
	}
	return packedTrx, nil
}

func (tx Tx) LastSignature() (xc.TxSignature, bool) {
	if len(tx.signatures) == 0 {
		return xc.TxSignature{}, false
	}
	return tx.signatures[len(tx.signatures)-1], true
}

func (tx Tx) SigCount() int {
	lastSig, ok := tx.LastSignature()
	if !ok {
		return 0
	}
	if IsCanonical(lastSig) {
		// use the count before we received the canonical signature
		return len(tx.signatures) - 1
	}
	return len(tx.signatures)
}
