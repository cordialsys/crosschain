package tx

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const sigHashMask = 0x1f

type sigHashExtFlag uint8
type taprootSigHashOptions struct {
	extFlag     sigHashExtFlag
	annexHash   []byte
	tapLeafHash []byte
	keyVersion  byte
	codeSepPos  uint32
}

// calcTaprootSignatureHashRaw computes the sighash as specified in BIP 143.
// If an invalid sighash type is passed in, an error is returned.
func calcTaprootSignatureHashRaw(sigHashes *txscript.TxSigHashes, hType txscript.SigHashType,
	tx *wire.MsgTx, idx int,
	prevOutFetcher txscript.PrevOutputFetcher,
) ([]byte, error) {

	opts := &taprootSigHashOptions{}

	// // If a valid sighash type isn't passed in, then we'll exit early.
	// if !isValidTaprootSigHash(hType) {
	// 	// TODO(roasbeef): use actual errr here
	// 	return nil, fmt.Errorf("invalid taproot sighash type: %v", hType)
	// }

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
		return nil, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// We'll utilize this buffer throughout to incrementally calculate
	// the signature hash for this transaction.
	var sigMsg bytes.Buffer

	// The final sighash always has a value of 0x00 prepended to it, which
	// is called the sighash epoch.
	sigMsg.WriteByte(0x00)

	// First, we write the hash type encoded as a single byte.
	if err := sigMsg.WriteByte(byte(hType)); err != nil {
		return nil, err
	}

	// Next we'll write out the transaction specific data which binds the
	// outer context of the sighash.
	err := binary.Write(&sigMsg, binary.LittleEndian, tx.Version)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&sigMsg, binary.LittleEndian, tx.LockTime)
	if err != nil {
		return nil, err
	}

	// If sighash isn't anyone can pay, then we'll include all the
	// pre-computed midstate digests in the sighash.
	if hType&txscript.SigHashAnyOneCanPay != txscript.SigHashAnyOneCanPay {
		sigMsg.Write(sigHashes.HashPrevOutsV1[:])
		sigMsg.Write(sigHashes.HashInputAmountsV1[:])
		sigMsg.Write(sigHashes.HashInputScriptsV1[:])
		sigMsg.Write(sigHashes.HashSequenceV1[:])
	}

	// If this is sighash all, or its taproot alias (sighash default),
	// then we'll also include the pre-computed digest of all the outputs
	// of the transaction.
	if hType&txscript.SigHashSingle != txscript.SigHashSingle &&
		hType&txscript.SigHashSingle != txscript.SigHashNone {

		sigMsg.Write(sigHashes.HashOutputsV1[:])
	}

	// Next, we'll write out the relevant information for this specific
	// input.
	//
	// The spend type is computed as the (ext_flag*2) + annex_present. We
	// use this to bind the extension flag (that BIP 342 uses), as well as
	// the annex if its present.
	input := tx.TxIn[idx]
	witnessHasAnnex := opts.annexHash != nil
	spendType := byte(opts.extFlag) * 2
	if witnessHasAnnex {
		spendType += 1
	}

	if err := sigMsg.WriteByte(spendType); err != nil {
		return nil, err
	}

	// If anyone can pay is active, then we'll write out just the specific
	// information about this input, given we skipped writing all the
	// information of all the inputs above.
	if hType&txscript.SigHashAnyOneCanPay == txscript.SigHashAnyOneCanPay {
		// We'll start out with writing this input specific information by
		// first writing the entire previous output.
		err = wire.WriteOutPoint(&sigMsg, 0, 0, &input.PreviousOutPoint)
		if err != nil {
			return nil, err
		}

		// Next, we'll write out the previous output (amt+script) being
		// spent itself.
		prevOut := prevOutFetcher.FetchPrevOutput(input.PreviousOutPoint)
		if err := wire.WriteTxOut(&sigMsg, 0, 0, prevOut); err != nil {
			return nil, err
		}

		// Finally, we'll write out the input sequence itself.
		err = binary.Write(&sigMsg, binary.LittleEndian, input.Sequence)
		if err != nil {
			return nil, err
		}
	} else {
		err := binary.Write(&sigMsg, binary.LittleEndian, uint32(idx))
		if err != nil {
			return nil, err
		}
	}

	// Now that we have the input specific information written, we'll
	// include the anex, if we have it.
	if witnessHasAnnex {
		sigMsg.Write(opts.annexHash)
	}

	// Finally, if this is sighash single, then we'll write out the
	// information for this given output.
	if hType&sigHashMask == txscript.SigHashSingle {
		// If this output doesn't exist, then we'll return with an error
		// here as this is an invalid sighash type for this input.
		if idx >= len(tx.TxOut) {
			// TODO(roasbeef): real error here
			return nil, fmt.Errorf("invalid sighash type for input")
		}

		// Now that we know this is a valid sighash input combination,
		// we'll write out the information specific to this input.
		// We'll write the wire serialization of the output and compute
		// the sha256 in a single step.
		shaWriter := sha256.New()
		txOut := tx.TxOut[idx]
		if err := wire.WriteTxOut(shaWriter, 0, 0, txOut); err != nil {
			return nil, err
		}

		// With the digest obtained, we'll write this out into our
		// signature message.
		if _, err := sigMsg.Write(shaWriter.Sum(nil)); err != nil {
			return nil, err
		}
	}

	// // Now that we've written out all the base information, we'll write any
	// // message extensions (if they exist).
	// if err := opts.writeDigestExtensions(&sigMsg); err != nil {
	// 	return nil, err
	// }

	// The final sighash is computed as: hash_TagSigHash(0x00 || sigMsg).
	// We wrote the 0x00 above so we don't need to append here and incur
	// extra allocations.
	return AddHashedPrefix(chainhash.TagTapSighash, sigMsg.Bytes()), nil
}

// like chainhash.TaggedHash, but without the final hash,
// so we can let a remote signer safely hash first.
func AddHashedPrefix(tag []byte, msgs ...[]byte) []byte {
	shaTag := sha256.Sum256(tag)
	buf := bytes.NewBuffer([]byte{})

	buf.Write(shaTag[:])
	buf.Write(shaTag[:])

	for _, msg := range msgs {
		buf.Write(msg)
	}

	return buf.Bytes()
}
