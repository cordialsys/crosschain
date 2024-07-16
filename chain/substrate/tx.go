package substrate

import (
	"fmt"
	"math"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic/extensions"
	xc "github.com/cordialsys/crosschain"
	"golang.org/x/crypto/blake2b"
)

// Tx for Template
type Tx struct {
	// extrinsic            types.Extrinsic
	extrinsic            extrinsic.DynamicExtrinsic
	meta                 Metadata
	sender               types.MultiAddress
	genesisHash, curHash types.Hash
	rv                   types.RuntimeVersion
	tip, nonce           uint64
	era                  uint16
	inputSignatures      []xc.TxSignature
	payload              *extrinsic.Payload
}

var _ xc.Tx = &Tx{}

func NewTx(extrinsic extrinsic.DynamicExtrinsic, sender types.MultiAddress, tip uint64, txInput *TxInput) (*Tx, error) {
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

	payload, err := createPayload(&tx.meta, encodedMethod)
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

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	ser, err := tx.Serialize()
	if err != nil {
		return xc.TxHash("")
	}
	hash := blake2b.Sum256(ser)
	return xc.TxHash(codec.HexEncodeToString(hash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	b, err := codec.Encode(tx.payload)
	// if data is longer than 256 bytes, must hash it first
	if len(b) > 256 {
		h := blake2b.Sum256(b)
		b = h[:]
	}
	return []xc.TxDataToSign{b}, err
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	tx.extrinsic.Signature = &extrinsic.Signature{
		Signer: tx.sender,
		Signature: types.MultiSignature{
			IsEd25519: true,
			AsEd25519: types.NewSignature(signatures[0]),
		},
		SignedFields: tx.payload.SignedFields,
	}
	tx.extrinsic.Version |= types.ExtrinsicBitSigned
	tx.inputSignatures = []xc.TxSignature{signatures[0]}
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
