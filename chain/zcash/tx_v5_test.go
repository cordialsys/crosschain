package zcash

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

type referenceVectorV5 struct {
	Name         string
	BranchID     uint32 `json:"branch_id"`
	LockTime     uint32 `json:"lock_time"`
	ExpiryHeight uint32 `json:"expiry_height"`
	Inputs       []struct {
		TxID         string
		Index        uint32
		Sequence     uint32
		Amount       uint64
		ScriptPubkey string `json:"script_pubkey"`
		ScriptSig    string `json:"script_sig"`
	}
	Outputs []struct {
		Amount       int64
		ScriptPubkey string `json:"script_pubkey"`
	}
	Serialized string
	TxID       string
	Sighashes  []string
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func loadReferenceV5(t *testing.T) []referenceVectorV5 {
	t.Helper()
	data, err := os.ReadFile("testdata/transparent_v5.json")
	require.NoError(t, err)
	var fixtures struct{ Vectors []referenceVectorV5 }
	require.NoError(t, json.Unmarshal(data, &fixtures))
	require.Len(t, fixtures.Vectors, 6)
	return fixtures.Vectors
}

func (v referenceVectorV5) transaction(t *testing.T) *ZcashTx {
	t.Helper()
	tx := &ZcashTx{
		Version: versionV5, VersionGroupId: versionGroupV5, ConsensusBranchId: v.BranchID,
		LockTime: v.LockTime, ExpiryHeight: v.ExpiryHeight, SigHash: txscript.SigHashAll,
	}
	for _, input := range v.Inputs {
		tx.Inputs = append(tx.Inputs, ZcashTxInput{
			TxID: decodeHex(t, input.TxID), Index: input.Index, NSequence: input.Sequence,
			Amount: input.Amount, PubkeyScript: decodeHex(t, input.ScriptPubkey),
			SignatureScript: decodeHex(t, input.ScriptSig),
		})
	}
	for _, output := range v.Outputs {
		tx.Outputs = append(tx.Outputs, ZcashTxOutput{
			Amount: output.Amount, ScriptPubkey: decodeHex(t, output.ScriptPubkey),
		})
	}
	return tx
}

func TestV5ReferenceVectors(t *testing.T) {
	for _, v := range loadReferenceV5(t) {
		t.Run(v.Name, func(t *testing.T) {
			tx := v.transaction(t)
			serialized, err := tx.Serialize()
			require.NoError(t, err)
			require.Equal(t, v.Serialized, hex.EncodeToString(serialized))
			hash, err := tx.Hash()
			require.NoError(t, err)
			require.Equal(t, v.TxID, hex.EncodeToString(hash))
			sighashes, err := tx.Sighashes()
			require.NoError(t, err)
			require.Len(t, sighashes, len(v.Sighashes))
			for i, expected := range v.Sighashes {
				require.Equal(t, expected, hex.EncodeToString(sighashes[i]))
			}
		})
	}
}

func TestV5SignatureCommitments(t *testing.T) {
	v := loadReferenceV5(t)[2] // three inputs and three outputs
	original := v.transaction(t)
	hashes, err := original.Sighashes()
	require.NoError(t, err)
	for name, mutate := range map[string]func(*ZcashTx){
		"other input amount": func(tx *ZcashTx) { tx.Inputs[2].Amount++ },
		"other input script": func(tx *ZcashTx) { tx.Inputs[2].PubkeyScript[5] ^= 1 },
		"other outpoint":     func(tx *ZcashTx) { tx.Inputs[2].Index++ },
		"other sequence":     func(tx *ZcashTx) { tx.Inputs[2].NSequence-- },
		"output amount":      func(tx *ZcashTx) { tx.Outputs[2].Amount++ },
		"output script":      func(tx *ZcashTx) { tx.Outputs[2].ScriptPubkey[5] ^= 1 },
		"branch":             func(tx *ZcashTx) { tx.ConsensusBranchId++ },
		"locktime":           func(tx *ZcashTx) { tx.LockTime++ },
		"expiry":             func(tx *ZcashTx) { tx.ExpiryHeight++ },
	} {
		t.Run(name, func(t *testing.T) {
			tx := v.transaction(t)
			mutate(tx)
			changed, err := tx.Sighashes()
			require.NoError(t, err)
			for i := range hashes {
				require.NotEqual(t, hashes[i], changed[i], "signature %d must commit to change", i)
			}
		})
	}
	// Authorizing data is deliberately excluded from txid and signature digests.
	originalHash, err := original.Hash()
	require.NoError(t, err)
	originalBytes, err := original.Serialize()
	require.NoError(t, err)
	original.Inputs[0].SignatureScript = []byte{1, 2, 3}
	newHash, err := original.Hash()
	require.NoError(t, err)
	require.Equal(t, originalHash, newHash)
	newHashes, err := original.Sighashes()
	require.NoError(t, err)
	require.Equal(t, hashes, newHashes)
	newBytes, err := original.Serialize()
	require.NoError(t, err)
	require.NotEqual(t, originalBytes, newBytes)
}

func TestV5RejectsMalformedInputs(t *testing.T) {
	v := loadReferenceV5(t)[2]
	for name, mutate := range map[string]func(*ZcashTx){
		"unsupported version": func(tx *ZcashTx) { tx.Version = 0x80000006 },
		"missing overwinter":  func(tx *ZcashTx) { tx.Version = 5 },
		"coinbase": func(tx *ZcashTx) {
			tx.Inputs[0].TxID = make([]byte, 32)
			tx.Inputs[0].Index = 0xffffffff
		},
		"missing branch":  func(tx *ZcashTx) { tx.ConsensusBranchId = 0 },
		"wrong group":     func(tx *ZcashTx) { tx.VersionGroupId = 0x892F2085 },
		"invalid expiry":  func(tx *ZcashTx) { tx.ExpiryHeight = 500_000_000 },
		"missing txid":    func(tx *ZcashTx) { tx.Inputs[0].TxID = nil },
		"short txid":      func(tx *ZcashTx) { tx.Inputs[0].TxID = make([]byte, 31) },
		"long txid":       func(tx *ZcashTx) { tx.Inputs[0].TxID = make([]byte, 33) },
		"duplicate input": func(tx *ZcashTx) { tx.Inputs[1] = tx.Inputs[0] },
		"input overflow":  func(tx *ZcashTx) { tx.Inputs[0].Amount = ^uint64(0) },
		"input total":     func(tx *ZcashTx) { tx.Inputs[0].Amount = maxMoney },
		"negative output": func(tx *ZcashTx) { tx.Outputs[0].Amount = -1 },
		"large output":    func(tx *ZcashTx) { tx.Outputs[0].Amount = int64(maxMoney) + 1 },
		"output total":    func(tx *ZcashTx) { tx.Outputs[0].Amount = int64(maxMoney) },
	} {
		t.Run(name, func(t *testing.T) {
			tx := v.transaction(t)
			mutate(tx)
			_, err := tx.Serialize()
			require.Error(t, err)
			_, err = tx.Hash()
			require.Error(t, err)
			_, err = tx.Sighashes()
			require.Error(t, err)
		})
	}
	for _, mode := range []txscript.SigHashType{0, txscript.SigHashNone, txscript.SigHashSingle, txscript.SigHashAll | txscript.SigHashAnyOneCanPay, 0x101} {
		tx := v.transaction(t)
		tx.SigHash = mode
		_, err := tx.Sighashes()
		require.ErrorContains(t, err, "only supports SIGHASH_ALL")
	}
}

func TestV5CompactSizeCounts(t *testing.T) {
	tx := loadReferenceV5(t)[1].transaction(t)
	input := tx.Inputs[0]
	output := tx.Outputs[0]
	tx.Inputs, tx.Outputs = nil, nil
	for i := 0; i < 253; i++ {
		input.Index = uint32(i)
		tx.Inputs = append(tx.Inputs, input)
		tx.Outputs = append(tx.Outputs, output)
	}
	serialized, err := tx.Serialize()
	require.NoError(t, err)
	r := bytes.NewReader(serialized[20:])
	n, err := wire.ReadVarInt(r, 0)
	require.NoError(t, err)
	require.EqualValues(t, 253, n)
	// These fixtures have empty scriptSigs, so each input is 41 bytes.
	_, err = io.CopyN(io.Discard, r, int64(41*len(tx.Inputs)))
	require.NoError(t, err)
	n, err = wire.ReadVarInt(r, 0)
	require.NoError(t, err)
	require.EqualValues(t, 253, n)
}
