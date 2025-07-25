package tx

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	btcec "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	log "github.com/sirupsen/logrus"
)

type Input struct {
	tx_input.Output `json:"output"`
	Address         xc.Address `json:"address,omitempty"`
}

type Recipient struct {
	To    xc.Address          `json:"to"`
	Value xc.AmountBlockchain `json:"value"`
}

// Tx for Bitcoin
type Tx struct {
	MsgTx      *wire.MsgTx
	Signed     bool
	Recipients []Recipient
	Signatures []xc.TxSignature

	UnspentOutputs []tx_input.Output `json:"unspent_outputs"`
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx *Tx) Hash() xc.TxHash {
	return tx.txHashReversed()
}

func (tx *Tx) txHashReversed() xc.TxHash {
	txHash := tx.txHashNormalBytes()

	size := len(txHash)
	txHashReversed := make([]byte, size)
	copy(txHashReversed[:], txHash[:])
	for i := 0; i < size/2; i++ {
		txHashReversed[i], txHashReversed[size-1-i] = txHashReversed[size-1-i], txHashReversed[i]
	}
	return xc.TxHash(hex.EncodeToString(txHashReversed))
}
func (tx *Tx) txHashNormal() xc.TxHash {
	txhash := tx.txHashNormalBytes()
	return xc.TxHash(hex.EncodeToString(txhash[:]))
}
func (tx *Tx) txHashNormalBytes() []byte {
	txhash := tx.MsgTx.TxHash()
	return txhash[:]
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	sighashes := make([]*xc.SignatureRequest, len(tx.UnspentOutputs))
	if len(tx.UnspentOutputs) == 0 {
		return sighashes, nil
	}

	// Taproot requires calculating the sum of all input values for
	// precomputed midstate sighashes. Because of this, we have to
	// prepare a proper mapped fetcher.
	mapping := make(map[wire.OutPoint]*wire.TxOut)
	for _, utxo := range tx.UnspentOutputs {
		op := wire.OutPoint{
			Hash:  chainhash.Hash(utxo.Outpoint.Hash),
			Index: utxo.Outpoint.Index,
		}
		mapping[op] = &wire.TxOut{
			Value:    int64(utxo.Value.Uint64()),
			PkScript: utxo.PubKeyScript,
		}
	}
	fetcher := txscript.NewMultiPrevOutFetcher(mapping)
	sighashMidstate := txscript.NewTxSigHashes(tx.MsgTx, fetcher)

	for i, utxo := range tx.UnspentOutputs {
		pubKeyScript := utxo.PubKeyScript
		value := utxo.Value.Uint64()

		var hash []byte
		var err error

		isTaproot := txscript.IsPayToTaproot(pubKeyScript)
		isSegWit := txscript.IsPayToWitnessPubKeyHash(pubKeyScript)
		log.Debugf("Sighashes params: IsPayToWitnessPubKeyHash(pubKeyScript)=%t, IsPayToTaproot(pubKeyScript)=%t", isSegWit, isTaproot)

		if isTaproot {
			log.Info("CalcTaprootSignatureHash")
			hash, err = txscript.CalcTaprootSignatureHash(
				sighashMidstate,
				txscript.SigHashDefault,
				tx.MsgTx,
				i,
				fetcher,
			)
		} else if isSegWit {
			log.Infof("CalcWitnessSigHash with pubKeyScript: %s", base64.RawURLEncoding.EncodeToString(pubKeyScript))
			hash, err = txscript.CalcWitnessSigHash(
				pubKeyScript,
				sighashMidstate,
				txscript.SigHashAll,
				tx.MsgTx,
				i,
				int64(value),
			)
		} else {
			log.Infof("CalcSignatureHash with pubKeyScript: %s", base64.RawURLEncoding.EncodeToString(pubKeyScript))
			hash, err = txscript.CalcSignatureHash(pubKeyScript, txscript.SigHashAll, tx.MsgTx, i)
		}

		if err != nil {
			return []*xc.SignatureRequest{}, err
		}

		sighashes[i] = xc.NewSignatureRequest(hash, utxo.Address)
	}

	return sighashes, nil
}

// returns (r, s, err)
func DecodeEcdsaSignature(signature xc.TxSignature) (btcec.ModNScalar, btcec.ModNScalar, error) {
	var err error
	var r btcec.ModNScalar
	var s btcec.ModNScalar
	rsv := [65]byte{}
	if len(signature) != 65 && len(signature) != 64 {
		return r, s, errors.New("signature must be 64 or 65 length serialized bytestring of r,s, and recovery byte")
	}
	copy(rsv[:], signature)

	// Decode the signature and the pubkey script.
	rInt := new(big.Int).SetBytes(rsv[:32])
	sInt := new(big.Int).SetBytes(rsv[32:64])

	rBz := r.Bytes()
	sBz := s.Bytes()
	rInt.FillBytes(rBz[:])
	sInt.FillBytes(sBz[:])
	r.SetBytes(&rBz)
	s.SetBytes(&sBz)
	return r, s, err
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatureResponses ...*xc.SignatureResponse) error {
	if tx.Signed {
		return fmt.Errorf("already signed")
	}
	signatures := make([]xc.TxSignature, len(signatureResponses))
	for i, signatureResponse := range signatureResponses {
		signatures[i] = signatureResponse.Signature
	}

	tx.Signatures = signatures
	if len(signatures) != len(tx.MsgTx.TxIn) {
		return fmt.Errorf("expected %v signatures, got %v signatures", len(tx.MsgTx.TxIn), len(signatures))
	}

	for i, rsvBytes := range signatures {
		r, s, err := DecodeEcdsaSignature(rsvBytes)
		if err != nil {
			return err
		}

		signature := ecdsa.NewSignature(&r, &s)
		pubKeyScript := tx.UnspentOutputs[i].PubKeyScript
		signatureWithSuffix := append(signature.Serialize(), byte(txscript.SigHashAll))

		if txscript.IsPayToTaproot(pubKeyScript) {
			// Taproot witness
			log.Debug("append signature (taproot)")
			tx.MsgTx.TxIn[i].Witness = wire.TxWitness{rsvBytes}
		} else if txscript.IsPayToWitnessPubKeyHash(pubKeyScript) || txscript.IsPayToWitnessScriptHash(pubKeyScript) {
			// Segwit witness
			log.Debug("append signature (segwit)")
			tx.MsgTx.TxIn[i].Witness = wire.TxWitness([][]byte{signatureWithSuffix, signatureResponses[i].PublicKey})
		} else {
			log.Debug("append signature (legacy)")
			// Support non-segwit
			builder := txscript.NewScriptBuilder()
			builder.AddData(signatureWithSuffix)
			builder.AddData(signatureResponses[i].PublicKey)
			tx.MsgTx.TxIn[i].SignatureScript, err = builder.Script()
			if err != nil {
				return err
			}
		}
	}

	tx.Signed = true
	return nil
}

func (tx *Tx) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := tx.MsgTx.Serialize(buf); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

// Outputs returns the UTXO outputs in the underlying transaction.
func (tx *Tx) Outputs() ([]tx_input.Output, error) {
	hash := tx.txHashNormal()
	outputs := make([]tx_input.Output, len(tx.MsgTx.TxOut))
	for i := range outputs {
		outputs[i].Outpoint = tx_input.Outpoint{
			Hash:  []byte(hash),
			Index: uint32(i),
		}
		outputs[i].PubKeyScript = tx.MsgTx.TxOut[i].PkScript
		if tx.MsgTx.TxOut[i].Value < 0 {
			return nil, fmt.Errorf("bad output %v: value is less than zero", i)
		}
		outputs[i].Value = xc.NewAmountBlockchainFromUint64(uint64(tx.MsgTx.TxOut[i].Value))
	}
	return outputs, nil
}

// Heuristic to determine the sender of a transaction by
// using the largest utxo input and taking it's spender.
func DetectFrom(inputs []Input) (string, xc.AmountBlockchain) {
	from := ""
	max := xc.NewAmountBlockchainFromUint64(0)
	totalIn := xc.NewAmountBlockchainFromUint64(0)
	for _, input := range inputs {
		value := input.Output.Value
		if value.Cmp(&max) > 0 {
			max = value
			from = string(input.Address)
		}
		// fmt.Println("inputfrom: ", input.Address)
		totalIn = totalIn.Add(&value)
	}
	return from, totalIn
}

func (tx *Tx) DetectToAndAmount(from string, expectedTo string) (string, xc.AmountBlockchain, xc.AmountBlockchain) {
	to := expectedTo
	amount := xc.NewAmountBlockchainFromUint64(0)
	totalOut := xc.NewAmountBlockchainFromUint64(0)

	for _, recipient := range tx.Recipients {
		addr := string(recipient.To)
		value := recipient.Value

		// if we know "to", we add the value(s)
		if expectedTo != "" && addr == expectedTo {
			amount = amount.Add(&value)
		}

		// fmt.Println("from: ", from, "to: ", to)
		// if we don't know "to", we set "to" as anything different than "from"
		if to == "" && addr != from {
			amount = value
			to = addr
		}

		totalOut = totalOut.Add(&value)
	}
	// fmt.Println("recipient to: ", to, amount.String())
	return to, amount, totalOut
}
