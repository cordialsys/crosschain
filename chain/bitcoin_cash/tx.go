package bitcoin_cash

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	log "github.com/sirupsen/logrus"
)

// Tx for Bitcoin
type Tx struct {
	*bitcoin.Tx
}

var _ xc.Tx = &Tx{}

// Sighashes returns the tx payload to sign, aka sighash
func (tx *Tx) Sighashes() ([]xc.TxDataToSign, error) {
	sighashes := make([]xc.TxDataToSign, len(tx.Input.UnspentOutputs))

	for i, utxo := range tx.Input.UnspentOutputs {
		txin := bitcoin.Input{
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
		log.Debugf("Sighashes params: sigScript=%s IsPayToWitnessPubKeyHash(pubKeyScript)=%t", base64.RawStdEncoding.EncodeToString(sigScript), txscript.IsPayToWitnessPubKeyHash(pubKeyScript))
		if sigScript == nil {
			hash = CalculateBchBip143Sighash(pubKeyScript, txscript.NewTxSigHashes(tx.MsgTx, fetcher), txscript.SigHashAll, tx.MsgTx, i, int64(value))
		} else {
			hash = CalculateBchBip143Sighash(sigScript, txscript.NewTxSigHashes(tx.MsgTx, fetcher), txscript.SigHashAll, tx.MsgTx, i, int64(value))
		}
		if err != nil {
			return []xc.TxDataToSign{}, err
		}

		sighashes[i] = hash
	}

	return sighashes, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.Signed {
		return fmt.Errorf("already signed")
	}
	if len(signatures) != len(tx.MsgTx.TxIn) {
		return fmt.Errorf("expected %v signatures, got %v signatures", len(tx.MsgTx.TxIn), len(signatures))
	}

	for i, rsvBytes := range signatures {
		r, s, err := bitcoin.DecodeEcdsaSignature(rsvBytes)
		if err != nil {
			return err
		}

		signature := ecdsa.NewSignature(&r, &s)
		// pubKeyScript := tx.Input.UnspentOutputs[i].PubKeyScript
		// var sigScript []byte = nil

		// // Support segwit.
		// if sigScript == nil {
		// 	if txscript.IsPayToWitnessPubKeyHash(pubKeyScript) || txscript.IsPayToWitnessScriptHash(pubKeyScript) {
		// 		log.Debug("append signature (segwit)")
		// 		tx.MsgTx.TxIn[i].Witness = wire.TxWitness([][]byte{append(signature.Serialize(), byte(txscript.SigHashAll)), tx.Input.FromPublicKey})
		// 		continue
		// 	}
		// } else {
		// 	if txscript.IsPayToWitnessScriptHash(sigScript) {
		// 		log.Debug("append signature + sigscript (segwit)")
		// 		tx.MsgTx.TxIn[i].Witness = wire.TxWitness([][]byte{append(signature.Serialize(), byte(txscript.SigHashAll)), tx.Input.FromPublicKey, sigScript})
		// 		continue
		// 	}
		// }

		// Support non-segwit
		builder := txscript.NewScriptBuilder()
		sigHashByte := txscript.SigHashAll
		sigHashByte = sigHashByte | SighashForkID
		builder.AddData(append(signature.Serialize(), byte(sigHashByte)))
		builder.AddData(tx.Input.FromPublicKey)
		log.Debug("append signature (non-segwit)")
		// if sigScript != nil {
		// 	log.Debug("append sigScript (non-segwit)")
		// 	builder.AddData(sigScript)
		// }
		tx.MsgTx.TxIn[i].SignatureScript, err = builder.Script()
		if err != nil {
			return err
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
