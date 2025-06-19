package tx

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
	ecc2 "github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/cordialsys/crosschain/chain/eos/tx/action"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
)

// Tx for Template
type Tx struct {
	chain      *xc.ChainBaseConfig
	args       xcbuilder.TransferArgs
	input      *tx_input.TxInput
	signatures []xc.TxSignature
}

var _ xc.Tx = &Tx{}

// It's necessary to have to keep requesting signatures repeatedly until one
// of EOS's 'canoncical' signatures are found.
var _ xc.TxAdditionalSighashes = &Tx{}

func NewTx(chain *xc.ChainBaseConfig, args xcbuilder.TransferArgs, input *tx_input.TxInput) *Tx {
	return &Tx{chain, args, input, []xc.TxSignature{}}
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	eosTx, err := tx.GetTransferTx()
	if err != nil {
		return ""
	}
	packedTrx, err := tx.SignAndPack(eosTx)
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
	eosTx, err := tx.GetTransferTx()
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
	eosTx, err := tx.GetTransferTx()
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
	eosTx, err := tx.GetTransferTx()
	if err != nil {
		return nil, err
	}
	packedTrx, err := tx.SignAndPack(eosTx)
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
func (tx Tx) GetTransferTx() (*eos.Transaction, error) {
	fromAccount := tx.input.FromAccount
	if identity, ok := tx.args.GetFromIdentity(); ok {
		fromAccount = identity
	}
	toAccount := string(tx.args.GetTo())
	if identity, ok := tx.args.GetToIdentity(); ok {
		toAccount = identity
	}
	decimals, ok := tx.args.GetDecimals()
	if !ok {
		decimals = 4
	}
	amount := tx.args.GetAmount()
	humanAmount := amount.ToHuman(int32(decimals))

	contract, ok := tx.args.GetContract()
	if !ok {
		contract = tx_input.DefaultContractId(tx.chain)
	}
	contractAccount, symbol, err := tx_input.ParseContractId(tx.chain, contract, tx.input)
	if err != nil {
		return nil, err
	}
	memo, _ := tx.args.GetMemo()

	action, err := action.NewTransfer(fromAccount, toAccount, humanAmount, int32(decimals), contractAccount, symbol, memo)
	if err != nil {
		return nil, err
	}
	// jsonPrint(action)
	eosTx := &eos.Transaction{Actions: []*eos.Action{action}}
	eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(tx.input.HeadBlockID[:4]))
	eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(tx.input.HeadBlockID[8:16])
	expiration := time.Unix(tx.input.Timestamp, 0)
	expiration = expiration.Add(tx_input.ExpirationPeriod)
	eosTx.Expiration = eos.JSONTime{Time: expiration}

	// The signer may be using deterministic signatures, so we need to make
	// some useless change in the signature body to force a completely different signature.
	eosTx.MaxCPUUsageMS = byte(tx.SigCount())

	return eosTx, nil
}

func (tx Tx) SignAndPack(eosTx *eos.Transaction) (*eos.PackedTransaction, error) {
	lastSig, ok := tx.LastSignature()
	if !ok {
		return nil, errors.New("EOS tx not signed")
	}
	signedTx := eos.NewSignedTransaction(eosTx)
	withPrefix := append([]byte{byte(ecc2.CurveK1)}, lastSig...)
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
