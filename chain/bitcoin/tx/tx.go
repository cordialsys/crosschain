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
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	log "github.com/sirupsen/logrus"
)

type Input struct {
	tx_input.Output `json:"output"`
	SigScript       []byte     `json:"sig_script,omitempty"`
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

	Amount xc.AmountBlockchain
	Input  *tx_input.TxInput
	From   xc.Address
	To     xc.Address
	// isBch  bool
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

func bzToString(bz []byte) string {
	return base64.RawStdEncoding.EncodeToString(bz)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx *Tx) Sighashes() ([]xc.TxDataToSign, error) {
	sighashes := make([]xc.TxDataToSign, len(tx.Input.UnspentOutputs))

	for i, utxo := range tx.Input.UnspentOutputs {
		txin := Input{
			Output: utxo,
		}
		pubKeyScript := txin.PubKeyScript
		sigScript := txin.SigScript
		value := txin.Value.Uint64()
		fetcher := txscript.NewCannedPrevOutputFetcher(
			pubKeyScript, int64(value),
		)

		var hash []byte
		var err error
		log.Debugf("Sighashes params: sigScript=%s IsPayToWitnessPubKeyHash(pubKeyScript)=%t", bzToString(sigScript), txscript.IsPayToWitnessPubKeyHash(pubKeyScript))
		if sigScript == nil {
			if txscript.IsPayToWitnessPubKeyHash(pubKeyScript) {
				log.Debugf("CalcWitnessSigHash with pubKeyScript: %s", base64.RawURLEncoding.EncodeToString(pubKeyScript))
				hash, err = txscript.CalcWitnessSigHash(pubKeyScript, txscript.NewTxSigHashes(tx.MsgTx, fetcher), txscript.SigHashAll, tx.MsgTx, i, int64(value))
			} else {
				log.Debugf("CalcSignatureHash with pubKeyScript: %s", base64.RawURLEncoding.EncodeToString(pubKeyScript))
				hash, err = txscript.CalcSignatureHash(pubKeyScript, txscript.SigHashAll, tx.MsgTx, i)
			}
		} else {
			if txscript.IsPayToWitnessScriptHash(pubKeyScript) {
				log.Debugf("CalcWitnessSigHash with sigScript: %s", base64.RawURLEncoding.EncodeToString(sigScript))
				hash, err = txscript.CalcWitnessSigHash(sigScript, txscript.NewTxSigHashes(tx.MsgTx, fetcher), txscript.SigHashAll, tx.MsgTx, i, int64(value))
			} else {
				log.Debugf("CalcSignatureHash with sigScript: %s", base64.RawURLEncoding.EncodeToString(sigScript))
				hash, err = txscript.CalcSignatureHash(sigScript, txscript.SigHashAll, tx.MsgTx, i)
			}
		}
		if err != nil {
			return []xc.TxDataToSign{}, err
		}

		sighashes[i] = hash
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

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.Signed {
		return fmt.Errorf("already signed")
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
		pubKeyScript := tx.Input.UnspentOutputs[i].PubKeyScript
		signatureWithSuffix := append(signature.Serialize(), byte(txscript.SigHashAll))

		// Support segwit.
		if txscript.IsPayToWitnessPubKeyHash(pubKeyScript) || txscript.IsPayToWitnessScriptHash(pubKeyScript) {
			log.Debug("append signature (segwit)")
			tx.MsgTx.TxIn[i].Witness = wire.TxWitness([][]byte{signatureWithSuffix, tx.Input.FromPublicKey})
			continue
		}

		// Support non-segwit
		builder := txscript.NewScriptBuilder()
		builder.AddData(signatureWithSuffix)
		builder.AddData(tx.Input.FromPublicKey)
		tx.MsgTx.TxIn[i].SignatureScript, err = builder.Script()
		if err != nil {
			return err
		}
	}

	tx.Signed = true
	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.Signatures
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
