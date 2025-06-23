package tx

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	xc "github.com/cordialsys/crosschain"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	ecc2 "github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
)

// Tx for Template
type Tx struct {
	chain              *xc.ChainBaseConfig
	input              *tx_input.TxInput
	signatures         []xc.TxSignature
	feePayerSignatures []xc.TxSignature
	builtTx            *eos.Transaction
	feePayer           xc.Address
}

var _ xc.Tx = &Tx{}

// It's necessary to have to keep requesting signatures repeatedly until one
// of EOS's 'canoncical' signatures are found.
var _ xc.TxAdditionalSighashes = &Tx{}

func NewTx(chain *xc.ChainBaseConfig, input *tx_input.TxInput, builtTx *eos.Transaction, feePayer xc.Address) *Tx {
	return &Tx{chain, input, []xc.TxSignature{}, []xc.TxSignature{}, builtTx, feePayer}
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	eosTx, err := tx.BuildTx()
	if err != nil {
		return ""
	}
	lastSigs, ok := tx.LastSignatures()
	if !ok {
		return ""
	}
	packedTrx, err := tx.SignAndPack(eosTx, lastSigs)
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

	requests := []*xc.SignatureRequest{
		xc.NewSignatureRequest(sigDigest),
	}
	if tx.feePayer != "" {
		requests = append(requests, xc.NewSignatureRequest(sigDigest, tx.feePayer))
	}
	return requests, nil
}

func (tx Tx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {

	// Have to keep trying...
	_, allCanonical := tx.SigCount()
	if allCanonical {
		// the search is over :)
		return nil, nil
	}

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

	requests := []*xc.SignatureRequest{
		xc.NewSignatureRequest(sigDigest),
	}
	if tx.feePayer != "" {
		requests = append(requests, xc.NewSignatureRequest(sigDigest, tx.feePayer))
	}
	return requests, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	tx.signatures = []xc.TxSignature{}
	for _, sig := range sigs {
		canonicalSigMaybe := SwapRecoveryByte(sig.Signature)
		if sig.Address == tx.feePayer && tx.feePayer != "" {
			tx.feePayerSignatures = append(tx.feePayerSignatures, canonicalSigMaybe)
		} else {
			tx.signatures = append(tx.signatures, canonicalSigMaybe)
		}
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
	lastSigs, ok := tx.LastSignatures()
	if !ok {
		return nil, errors.New("EOS tx not signed")
	}

	packedTrx, err := tx.SignAndPack(eosTx, lastSigs)
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

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) BuildTx() (*eos.Transaction, error) {
	eosTx := *tx.builtTx
	// The signer may be using deterministic signatures, so we need to make
	// some useless change in the signature body to force a completely different signature.
	sigCount, _ := tx.SigCount()
	seconds := time.Duration(sigCount) * time.Second
	eosTx.Expiration = eos.JSONTime{Time: eosTx.Expiration.Add(seconds)}
	return &eosTx, nil
}

func packSignature(sig xc.TxSignature) (ecc2.Signature, error) {
	withPrefix := append([]byte{byte(ecc2.CurveK1)}, sig...)
	sigFormatted, err := ecc2.NewSignatureFromData(withPrefix)
	if err != nil {
		return ecc2.Signature{}, err
	}
	return sigFormatted, nil
}

func (tx Tx) SignAndPack(eosTx *eos.Transaction, signatures []xc.TxSignature) (*eos.PackedTransaction, error) {
	signedTx := eos.NewSignedTransaction(eosTx)
	for _, signature := range signatures {
		sigFormatted, err := packSignature(signature)
		if err != nil {
			return nil, err
		}
		signedTx.Signatures = append(signedTx.Signatures, sigFormatted)
	}
	packedTrx, err := signedTx.Pack(eos.CompressionNone)
	if err != nil {
		return nil, err
	}
	return packedTrx, nil
}

func (tx Tx) LastSignatures() ([]xc.TxSignature, bool) {
	if len(tx.signatures) == 0 {
		return []xc.TxSignature{}, false
	}
	if tx.feePayer != "" {
		if len(tx.feePayerSignatures) == 0 {
			return []xc.TxSignature{}, false
		}

		return []xc.TxSignature{
			tx.signatures[len(tx.signatures)-1],
			tx.feePayerSignatures[len(tx.feePayerSignatures)-1],
		}, true
	} else {
		return []xc.TxSignature{
			tx.signatures[len(tx.signatures)-1],
		}, true
	}
}

func (tx Tx) SigCount() (int, bool) {
	lastSigs, ok := tx.LastSignatures()
	if !ok {
		return 0, false
	}
	all := true
	for _, sig := range lastSigs {
		if !IsCanonical(sig) {
			all = false
		}
	}
	if all {
		// use the count before we received the canonical signature(s)
		return len(tx.signatures) - 1, true
	}
	return len(tx.signatures), false
}
