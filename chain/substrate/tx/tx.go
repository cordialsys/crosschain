package tx

import (
	"fmt"
	"math"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic/extensions"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	"golang.org/x/crypto/blake2b"
)

// How many blocks the transaction will stay valid for
const MORTAL_PERIOD = 4096

// Tx for Template
type Tx struct {
	// extrinsic            types.Extrinsic
	extrinsic            extrinsic.DynamicExtrinsic
	meta                 tx_input.Metadata
	sender               types.MultiAddress
	genesisHash, curHash types.Hash
	rv                   types.RuntimeVersion
	tip, nonce           uint64
	era                  uint16
	inputSignatures      []xc.TxSignature
	payload              *extrinsic.Payload
}

var _ xc.Tx = &Tx{}

func NewTx(extrinsic extrinsic.DynamicExtrinsic, sender types.MultiAddress, tip uint64, txInput *tx_input.TxInput) (*Tx, error) {
	tx := &Tx{
		// extrinsic:   types.NewExtrinsic(call),
		meta:        txInput.Meta,
		extrinsic:   extrinsic,
		sender:      sender,
		nonce:       txInput.Nonce,
		genesisHash: txInput.GenesisHash,
		curHash:     txInput.CurHash,
		rv:          txInput.Rv,
		tip:         tip,
		era:         uint16(txInput.CurrentHeight%MORTAL_PERIOD<<4) + uint16(math.Log2(MORTAL_PERIOD)-1),
	}
	err := tx.build()
	return tx, err
}

func (tx *Tx) build() error {
	// tx.extrinsic.Sign()
	if tx.extrinsic.Type() != types.ExtrinsicVersion4 {
		return fmt.Errorf("unsupported extrinsic version: %v (isSigned: %v, type: %v)", tx.extrinsic.Version, tx.extrinsic.IsSigned(), tx.extrinsic.Type())
	}
	encodedMethod, err := codec.Encode(tx.extrinsic.Method)
	if err != nil {
		return fmt.Errorf("encode method: %w", err)
	}
	fieldValues := extrinsic.SignedFieldValues{}

	opts := []extrinsic.SigningOption{
		extrinsic.WithEra(types.ExtrinsicEra{IsImmortalEra: true}, tx.genesisHash),
		extrinsic.WithNonce(types.NewUCompactFromUInt(uint64(tx.nonce))),
		extrinsic.WithTip(types.NewUCompactFromUInt(tx.tip)),
		extrinsic.WithSpecVersion(tx.rv.SpecVersion),
		extrinsic.WithTransactionVersion(tx.rv.TransactionVersion),
		extrinsic.WithGenesisHash(tx.genesisHash),
		extrinsic.WithMetadataMode(extensions.CheckMetadataModeDisabled, extensions.CheckMetadataHash{Hash: types.NewEmptyOption[types.H256]()}),
	}
	for _, opt := range opts {
		opt(fieldValues)
	}

	payload, err := tx_input.CreatePayload(&tx.meta, encodedMethod)
	if err != nil {
		return fmt.Errorf("creating payload: %w", err)
	}
	err = payload.MutateSignedFields(fieldValues)
	if err != nil {
		return fmt.Errorf("mutate signed fields: %w", err)
	}
	tx.payload = payload
	return nil
}

func HashSerialized(serialized []byte) []byte {
	hash := blake2b.Sum256(serialized)
	return hash[:]
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	ser, err := tx.Serialize()
	if err != nil {
		return xc.TxHash("")
	}
	hash := HashSerialized(ser)
	return xc.TxHash(codec.HexEncodeToString(hash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	b, err := codec.Encode(tx.payload)
	// if data is longer than 256 bytes, must hash it first
	if len(b) > 256 {
		h := blake2b.Sum256(b)
		b = h[:]
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(b)}, err
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	tx.extrinsic.Signature = &extrinsic.Signature{
		Signer: tx.sender,
		Signature: types.MultiSignature{
			IsEd25519: true,
			AsEd25519: types.NewSignature(signatures[0].Signature),
		},
		SignedFields: tx.payload.SignedFields,
	}
	tx.extrinsic.Version |= types.ExtrinsicBitSigned
	tx.inputSignatures = []xc.TxSignature{signatures[0].Signature}
	// logrus.WithField("signature", hex.EncodeToString(signatures[0])).Debug("set signature")
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.inputSignatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return codec.Encode(tx.extrinsic)
}
