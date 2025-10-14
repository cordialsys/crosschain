package zcash

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	bitcointx "github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/harshavardhana/blake2b-simd"
)

// Tx for Bitcoin
type Tx struct {
	*tx.Tx
	signatures []*xc.SignatureResponse
}

func NewTx(tx *tx.Tx) *Tx {
	return &Tx{Tx: tx}
}

var _ xc.Tx = &tx.Tx{}

type ZcashTxInput struct {
	TxID         []byte
	PubkeyScript []byte
	Index        uint32
	NSequence    uint32
	Amount       uint64

	// Added when the signature is known
	SignatureScript []byte
}

func (input *ZcashTxInput) SerializeOutpoint() []byte {
	buf := new(bytes.Buffer)
	buf.Write(input.TxID)
	binary.Write(buf, binary.LittleEndian, input.Index)
	return buf.Bytes()
}

type ZcashTxOutput struct {
	Amount       int64
	ScriptPubkey []byte
}

func (output *ZcashTxOutput) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, output.Amount)

	err := wire.WriteVarBytes(buf, 0, output.ScriptPubkey)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// All inputs to match Zcash transaction format + hashing: https://zips.z.cash/zip-0243
type ZcashTx struct {
	Version           uint32
	VersionGroupId    uint32
	Inputs            []ZcashTxInput
	Outputs           []ZcashTxOutput
	LockTime          uint32
	ExpiryHeight      uint32
	SigHash           txscript.SigHashType
	ConsensusBranchId uint32
}

// ZIP-0243
func (tx *ZcashTx) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Header: Version (4 bytes)
	binary.Write(buf, binary.LittleEndian, tx.Version)

	// Header: VersionGroupId (4 bytes)
	binary.Write(buf, binary.LittleEndian, tx.VersionGroupId)

	// Transparent inputs: count (varint)
	err := wire.WriteVarInt(buf, 0, uint64(len(tx.Inputs)))
	if err != nil {
		return nil, fmt.Errorf("failed to write input count: %w", err)
	}

	// Transparent inputs: each input
	for _, input := range tx.Inputs {
		// Prevout: hash (32 bytes)
		buf.Write(input.TxID)
		// Prevout: index (4 bytes)
		binary.Write(buf, binary.LittleEndian, input.Index)
		err := wire.WriteVarBytes(buf, 0, input.SignatureScript)
		if err != nil {
			return nil, fmt.Errorf("failed to write input script: %w", err)
		}
		// Sequence (4 bytes)
		binary.Write(buf, binary.LittleEndian, input.NSequence)
	}

	// Transparent outputs: count (varint)
	err = wire.WriteVarInt(buf, 0, uint64(len(tx.Outputs)))
	if err != nil {
		return nil, fmt.Errorf("failed to write output count: %w", err)
	}

	// Transparent outputs: each output
	for _, output := range tx.Outputs {
		serialized, err := output.Serialize()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize output: %w", err)
		}
		buf.Write(serialized)
	}

	// LockTime (4 bytes)
	binary.Write(buf, binary.LittleEndian, tx.LockTime)

	// ExpiryHeight (4 bytes)
	binary.Write(buf, binary.LittleEndian, tx.ExpiryHeight)

	// ValueBalance (8 bytes, int64) - always 0 for transparent-only transactions
	binary.Write(buf, binary.LittleEndian, int64(0))

	// nShieldedSpend (varint) - 0 for transparent-only transactions
	err = wire.WriteVarInt(buf, 0, uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to write shielded spend count: %w", err)
	}

	// nShieldedOutput (varint) - 0 for transparent-only transactions
	err = wire.WriteVarInt(buf, 0, uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to write shielded output count: %w", err)
	}

	// nJoinSplit (varint) - 0 for transparent-only transactions
	err = wire.WriteVarInt(buf, 0, uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to write joinsplit count: %w", err)
	}

	return buf.Bytes(), nil
}

func (tx *ZcashTx) Hash() ([]byte, error) {
	serialized, err := tx.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	firstHash := sha256.Sum256(serialized)
	secondHash := sha256.Sum256(firstHash[:])

	// Reverse the bytes to get the display format (little-endian to big-endian)
	txid := make([]byte, len(secondHash))
	copy(txid, secondHash[:])
	for i, j := 0, len(txid)-1; i < j; i, j = i+1, j-1 {
		txid[i], txid[j] = txid[j], txid[i]
	}

	return txid, nil
}

func blake2bConfig(person []byte) *blake2b.Config {
	return &blake2b.Config{
		Size:   32,
		Person: person,
	}
}

func (tx *ZcashTx) Sighashes() ([][]byte, error) {
	hashPrevouts, err := blake2b.New(blake2bConfig([]byte("ZcashPrevoutHash")))
	if err != nil {
		return nil, err
	}
	hashSequence, err := blake2b.New(blake2bConfig([]byte("ZcashSequencHash")))
	if err != nil {
		return nil, err
	}
	hashOutputs, err := blake2b.New(blake2bConfig([]byte("ZcashOutputsHash")))
	if err != nil {
		return nil, err
	}
	for _, input := range tx.Inputs {
		hashPrevouts.Write(input.SerializeOutpoint())
		var bSequence [4]byte
		binary.LittleEndian.PutUint32(bSequence[:], input.NSequence)
		hashSequence.Write(bSequence[:])
	}

	for _, output := range tx.Outputs {
		serialized, err := output.Serialize()
		if err != nil {
			return nil, err
		}
		hashOutputs.Write(serialized)
	}
	hashPrevoutsBz := hashPrevouts.Sum(nil)
	hashSequenceBz := hashSequence.Sum(nil)
	hashOutputsBz := hashOutputs.Sum(nil)
	// all zeros since it's not used
	hashJoinSplitBz := make([]byte, 32)
	hashShieldedSpendsBz := make([]byte, 32)
	hashShieldedOutputsBz := make([]byte, 32)

	sighashes := [][]byte{}

	for _, input := range tx.Inputs {
		key := []byte("ZcashSigHash")
		key = binary.LittleEndian.AppendUint32(key, tx.ConsensusBranchId)

		hashOutputs, err := blake2b.New(blake2bConfig(key))
		if err != nil {
			return nil, err
		}

		// Write all of the transaction information
		binary.Write(hashOutputs, binary.LittleEndian, tx.Version)
		binary.Write(hashOutputs, binary.LittleEndian, tx.VersionGroupId)
		hashOutputs.Write(hashPrevoutsBz)
		hashOutputs.Write(hashSequenceBz)
		hashOutputs.Write(hashOutputsBz)
		hashOutputs.Write(hashJoinSplitBz)
		hashOutputs.Write(hashShieldedSpendsBz)
		hashOutputs.Write(hashShieldedOutputsBz)
		binary.Write(hashOutputs, binary.LittleEndian, tx.LockTime)
		binary.Write(hashOutputs, binary.LittleEndian, tx.ExpiryHeight)
		binary.Write(hashOutputs, binary.LittleEndian, int64(0)) //value balance
		binary.Write(hashOutputs, binary.LittleEndian, tx.SigHash)

		// Write the input specific information
		hashOutputs.Write(input.SerializeOutpoint())
		err = wire.WriteVarBytes(hashOutputs, 0, input.PubkeyScript)
		if err != nil {
			return nil, err
		}
		binary.Write(hashOutputs, binary.LittleEndian, input.Amount)
		binary.Write(hashOutputs, binary.LittleEndian, input.NSequence)

		sighashes = append(sighashes, hashOutputs.Sum(nil))
	}

	return sighashes, nil
}

func (tx *Tx) Build() (*ZcashTx, error) {
	ztx := &ZcashTx{
		// Version 4 (Sapling) with overwintered bit set
		Version: 4 | (0x80000000),
		// Sapling version group ID
		VersionGroupId: 0x892F2085,
		Inputs:         make([]ZcashTxInput, len(tx.UnspentOutputs)),
		Outputs:        make([]ZcashTxOutput, len(tx.Recipients)),
		// No locktime
		LockTime: 0,
		// No expiry
		ExpiryHeight:      0,
		SigHash:           txscript.SigHashAll,
		ConsensusBranchId: tx.Zcash.ConsensusBranchId,
	}

	for i, utxo := range tx.UnspentOutputs {
		ztx.Inputs[i] = ZcashTxInput{
			TxID:         utxo.Outpoint.Hash,
			Index:        utxo.Outpoint.Index,
			PubkeyScript: utxo.PubKeyScript,
			Amount:       utxo.Value.Uint64(),
			// Set to all ff -- indicate we do not use locktime.
			NSequence: 0xFFFFFFFF,
		}
		if len(tx.signatures) > i {
			sig := tx.signatures[i].Signature
			r, s, err := bitcointx.DecodeEcdsaSignature(sig)
			if err != nil {
				return nil, fmt.Errorf("failed to decode ecdsa signature: %w", err)
			}
			signatureDer := ecdsa.NewSignature(&r, &s).Serialize()

			// Build P2PKH signature script using txscript builder
			builder := txscript.NewScriptBuilder()
			// Add signature with sighash byte appended
			sigWithHashType := append(signatureDer, byte(ztx.SigHash))
			builder.AddData(sigWithHashType)
			// Add public key
			builder.AddData(tx.signatures[i].PublicKey)

			signatureScript, err := builder.Script()
			if err != nil {
				return nil, fmt.Errorf("failed to build signature script: %w", err)
			}
			ztx.Inputs[i].SignatureScript = signatureScript
		}
	}
	for i, out := range tx.MsgTx.TxOut {
		ztx.Outputs[i] = ZcashTxOutput{
			Amount:       out.Value,
			ScriptPubkey: out.PkScript,
		}
	}
	return ztx, nil
}

func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	ztx, err := tx.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}
	sighashes, err := ztx.Sighashes()
	if err != nil {
		return nil, fmt.Errorf("failed to compute sighashes: %w", err)
	}

	requests := make([]*xc.SignatureRequest, len(sighashes))
	for i, sighash := range sighashes {
		requests[i] = xc.NewSignatureRequest(sighash, tx.UnspentOutputs[i].Address)
	}

	return requests, nil
}

func (tx *Tx) Hash() xc.TxHash {
	ztx, err := tx.Build()
	if err != nil {
		return xc.TxHash("")
	}
	txid, err := ztx.Hash()
	if err != nil {
		return xc.TxHash("")
	}
	return xc.TxHash(hex.EncodeToString(txid))
}

func (tx *Tx) Serialize() ([]byte, error) {
	ztx, err := tx.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction: %w", err)
	}
	return ztx.Serialize()
}

func (tx *Tx) SetSignatures(signatureResponses ...*xc.SignatureResponse) error {
	if len(signatureResponses) != len(tx.UnspentOutputs) {
		return fmt.Errorf("expected %d signatures, got %d", len(tx.UnspentOutputs), len(signatureResponses))
	}
	tx.signatures = signatureResponses
	return nil
}
