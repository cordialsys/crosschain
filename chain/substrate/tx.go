package substrate

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	xc "github.com/cordialsys/crosschain"
	"golang.org/x/crypto/blake2b"
)

// Tx for Template
type Tx struct {
	extrinsic            types.Extrinsic
	sender               types.MultiAddress
	genesisHash, curHash types.Hash
	rv                   types.RuntimeVersion
	tip, nonce           uint64
	era                  uint16
	inputSignatures      []xc.TxSignature
}

var _ xc.Tx = &Tx{}

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
	mb, err := codec.Encode(tx.extrinsic.Method)
	if err != nil {
		return []xc.TxDataToSign{}, err
	}

	payload := types.ExtrinsicPayloadV4{
		ExtrinsicPayloadV3: types.ExtrinsicPayloadV3{
			Method: mb,
			Era: types.ExtrinsicEra{
				IsMortalEra: true,
				AsMortalEra: types.MortalEra{
					First:  byte(tx.era & 0xff),
					Second: byte(tx.era >> 8),
				},
			},
			Nonce:       types.NewUCompactFromUInt(tx.nonce),
			Tip:         types.NewUCompactFromUInt(tx.tip),
			SpecVersion: tx.rv.SpecVersion,
			GenesisHash: tx.genesisHash,
			BlockHash:   tx.curHash,
		},
		TransactionVersion: tx.rv.TransactionVersion,
	}

	b, err := codec.Encode(payload)
	return []xc.TxDataToSign{b}, err
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	tx.extrinsic.Signature = types.ExtrinsicSignatureV4{
		Signer: tx.sender,
		Signature: types.MultiSignature{
			IsSr25519: true,
			AsSr25519: types.NewSignature(signatures[0]),
		},
		Era: types.ExtrinsicEra{
			IsMortalEra: true,
			AsMortalEra: types.MortalEra{
				First:  byte(tx.era & 0xff),
				Second: byte(tx.era >> 8),
			},
		},
		Nonce: types.NewUCompactFromUInt(tx.nonce),
		Tip:   types.NewUCompactFromUInt(tx.tip),
	}
	tx.extrinsic.Version |= 0x80
	tx.inputSignatures = []xc.TxSignature{signatures[0]}
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.inputSignatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return codec.Encode(tx.extrinsic)
}
