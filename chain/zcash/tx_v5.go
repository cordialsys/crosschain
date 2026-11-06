package zcash

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/harshavardhana/blake2b-simd"
)

// ConsensusBranchIDNU7 applies to both public networks after NU7 activation.
// https://zips.z.cash/zip-0259
const ConsensusBranchIDNU7 uint32 = 0x77190AD9

const (
	versionV5      uint32 = 0x80000005
	versionGroupV5 uint32 = 0x26A7270A
	maxMoney       uint64 = 21_000_000 * 100_000_000
)

func (tx *ZcashTx) validateV5() error {
	if tx.Version != versionV5 || tx.VersionGroupId != versionGroupV5 {
		return fmt.Errorf("invalid Zcash v5 version or version group ID")
	}
	if tx.ConsensusBranchId == 0 {
		return fmt.Errorf("missing Zcash consensus branch ID")
	}
	if tx.ExpiryHeight >= 500_000_000 {
		return fmt.Errorf("Zcash expiry height is out of range")
	}
	var totalIn, totalOut uint64
	seen := make(map[[36]byte]bool, len(tx.Inputs))
	for i, input := range tx.Inputs {
		if len(input.TxID) != 32 {
			return fmt.Errorf("input %d transaction ID must be 32 bytes", i)
		}
		if input.Index == 0xffffffff && bytes.Equal(input.TxID, make([]byte, 32)) {
			return fmt.Errorf("coinbase inputs are not supported")
		}
		var outpoint [36]byte
		copy(outpoint[:32], input.TxID)
		binary.LittleEndian.PutUint32(outpoint[32:], input.Index)
		if seen[outpoint] {
			return fmt.Errorf("duplicate input %d", i)
		}
		seen[outpoint] = true
		if input.Amount > maxMoney-totalIn {
			return fmt.Errorf("input %d amount or total input value is out of range", i)
		}
		totalIn += input.Amount
	}
	for i, output := range tx.Outputs {
		if output.Amount < 0 || uint64(output.Amount) > maxMoney-totalOut {
			return fmt.Errorf("output %d amount or total output value is out of range", i)
		}
		totalOut += uint64(output.Amount)
	}
	return nil
}

// headerV5 is shared by the wire encoding and ZIP 244 header digest.
func (tx *ZcashTx) headerV5() []byte {
	header := make([]byte, 0, 20)
	for _, value := range []uint32{tx.Version, tx.VersionGroupId, tx.ConsensusBranchId, tx.LockTime, tx.ExpiryHeight} {
		header = binary.LittleEndian.AppendUint32(header, value)
	}
	return header
}

func (tx *ZcashTx) serializeV5() ([]byte, error) {
	if err := tx.validateV5(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(tx.headerV5())
	if err := wire.WriteVarInt(buf, 0, uint64(len(tx.Inputs))); err != nil {
		return nil, err
	}
	for _, input := range tx.Inputs {
		outpoint, err := input.SerializeOutpoint()
		if err != nil {
			return nil, err
		}
		buf.Write(outpoint)
		if err := wire.WriteVarBytes(buf, 0, input.SignatureScript); err != nil {
			return nil, err
		}
		buf.Write(binary.LittleEndian.AppendUint32(nil, input.NSequence))
	}
	if err := wire.WriteVarInt(buf, 0, uint64(len(tx.Outputs))); err != nil {
		return nil, err
	}
	for _, output := range tx.Outputs {
		encoded, err := output.Serialize()
		if err != nil {
			return nil, err
		}
		buf.Write(encoded)
	}
	// Empty Sapling spends, Sapling outputs, and Orchard actions. ZIP 225
	// omits their conditional fields, including value balances, entirely.
	buf.Write([]byte{0, 0, 0})
	return buf.Bytes(), nil
}

func digestV5(person string, fields ...[]byte) ([]byte, error) {
	h, err := blake2b.New(blake2bConfig([]byte(person)))
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		h.Write(field)
	}
	return h.Sum(nil), nil
}

type digestsV5 struct {
	header, prevouts, sequences, outputs, amounts, scripts, sapling, orchard []byte
}

// componentsV5 computes the shared ZIP 244 subtrees once, so signing many
// inputs remains linear in the size of the transaction.
func (tx *ZcashTx) componentsV5() (*digestsV5, error) {
	if err := tx.validateV5(); err != nil {
		return nil, err
	}
	var prevouts, sequences, outputs, amounts, scripts bytes.Buffer
	for _, input := range tx.Inputs {
		outpoint, err := input.SerializeOutpoint()
		if err != nil {
			return nil, err
		}
		prevouts.Write(outpoint)
		sequences.Write(binary.LittleEndian.AppendUint32(nil, input.NSequence))
		amounts.Write(binary.LittleEndian.AppendUint64(nil, input.Amount))
		if err := wire.WriteVarBytes(&scripts, 0, input.PubkeyScript); err != nil {
			return nil, err
		}
	}
	for _, output := range tx.Outputs {
		encoded, err := output.Serialize()
		if err != nil {
			return nil, err
		}
		outputs.Write(encoded)
	}
	d := new(digestsV5)
	for _, component := range []struct {
		person string
		data   []byte
		result *[]byte
	}{
		{"ZTxIdHeadersHash", tx.headerV5(), &d.header},
		{"ZTxIdPrevoutHash", prevouts.Bytes(), &d.prevouts},
		{"ZTxIdSequencHash", sequences.Bytes(), &d.sequences},
		{"ZTxIdOutputsHash", outputs.Bytes(), &d.outputs},
		{"ZTxTrAmountsHash", amounts.Bytes(), &d.amounts},
		{"ZTxTrScriptsHash", scripts.Bytes(), &d.scripts},
		// Empty shielded pools use personalized empty hashes, not zero bytes.
		{"ZTxIdSaplingHash", nil, &d.sapling},
		{"ZTxIdOrchardHash", nil, &d.orchard},
	} {
		value, err := digestV5(component.person, component.data)
		if err != nil {
			return nil, err
		}
		*component.result = value
	}
	return d, nil
}

func (tx *ZcashTx) rootDigestV5(d *digestsV5, transparent []byte) ([]byte, error) {
	person := binary.LittleEndian.AppendUint32([]byte("ZcashTxHash_"), tx.ConsensusBranchId)
	return digestV5(string(person), d.header, transparent, d.sapling, d.orchard)
}

func (tx *ZcashTx) hashV5() ([]byte, error) {
	d, err := tx.componentsV5()
	if err != nil {
		return nil, err
	}
	var fields [][]byte
	if len(tx.Inputs)+len(tx.Outputs) > 0 {
		fields = [][]byte{d.prevouts, d.sequences, d.outputs}
	}
	transparent, err := digestV5("ZTxIdTranspaHash", fields...)
	if err != nil {
		return nil, err
	}
	hash, err := tx.rootDigestV5(d, transparent)
	if err != nil {
		return nil, err
	}
	// RPC/display byte order is reversed; signature digests are not reversed.
	for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
		hash[i], hash[j] = hash[j], hash[i]
	}
	return hash, nil
}

func (tx *ZcashTx) sighashesV5() ([][]byte, error) {
	// Crosschain transfers commit to every input and output. Fail closed for
	// other flags rather than silently signing them with SIGHASH_ALL semantics.
	if tx.SigHash != txscript.SigHashAll {
		return nil, fmt.Errorf("Zcash v5 only supports SIGHASH_ALL")
	}
	d, err := tx.componentsV5()
	if err != nil {
		return nil, err
	}
	hashes := make([][]byte, len(tx.Inputs))
	for i, input := range tx.Inputs {
		outpoint, err := input.SerializeOutpoint()
		if err != nil {
			return nil, err
		}
		buf := bytes.NewBuffer(outpoint)
		buf.Write(binary.LittleEndian.AppendUint64(nil, input.Amount))
		if err := wire.WriteVarBytes(buf, 0, input.PubkeyScript); err != nil {
			return nil, err
		}
		buf.Write(binary.LittleEndian.AppendUint32(nil, input.NSequence))
		txin, err := digestV5("Zcash___TxInHash", buf.Bytes())
		if err != nil {
			return nil, err
		}
		transparent, err := digestV5("ZTxIdTranspaHash", []byte{byte(tx.SigHash)},
			d.prevouts, d.amounts, d.scripts, d.sequences, d.outputs, txin)
		if err != nil {
			return nil, err
		}
		hashes[i], err = tx.rootDigestV5(d, transparent)
		if err != nil {
			return nil, err
		}
	}
	return hashes, nil
}
