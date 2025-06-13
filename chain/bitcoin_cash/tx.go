package bitcoin_cash

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	log "github.com/sirupsen/logrus"
)

// Tx for Bitcoin
type Tx struct {
	*tx.Tx
}

func NewTx(tx *tx.Tx) *Tx {
	return &Tx{Tx: tx}
}

var _ xc.Tx = &tx.Tx{}

// Sighashes returns the tx payload to sign, aka sighash
func (txObj *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	sighashes := make([]*xc.SignatureRequest, len(txObj.UnspentOutputs))

	for i, utxo := range txObj.UnspentOutputs {
		pubKeyScript := utxo.PubKeyScript
		value := utxo.Value.Uint64()
		fetcher := txscript.NewCannedPrevOutputFetcher(
			pubKeyScript, int64(value),
		)

		var hash []byte
		log.Debugf("Sighashes params: IsPayToWitnessPubKeyHash(pubKeyScript)=%t", txscript.IsPayToWitnessPubKeyHash(pubKeyScript))
		hash = CalculateBchBip143Sighash(pubKeyScript, txscript.NewTxSigHashes(txObj.MsgTx, fetcher), txscript.SigHashAll, txObj.MsgTx, i, int64(value))

		sighashes[i] = xc.NewSignatureRequest(hash)
	}

	return sighashes, nil
}

// SetSignatures adds a signature to Tx
func (txObj *Tx) SetSignatures(signatureResponses ...*xc.SignatureResponse) error {
	if txObj.Signed {
		return fmt.Errorf("already signed")
	}
	if len(signatureResponses) != len(txObj.MsgTx.TxIn) {
		return fmt.Errorf("expected %v signatures, got %v signatures", len(txObj.MsgTx.TxIn), len(signatureResponses))
	}

	for i, signatureResponse := range signatureResponses {
		rsvBytes := signatureResponse.Signature
		r, s, err := tx.DecodeEcdsaSignature(rsvBytes)
		if err != nil {
			return err
		}

		signature := ecdsa.NewSignature(&r, &s)

		// Support non-segwit
		builder := txscript.NewScriptBuilder()
		sigHashByte := txscript.SigHashAll
		sigHashByte = sigHashByte | SighashForkID
		builder.AddData(append(signature.Serialize(), byte(sigHashByte)))
		builder.AddData(signatureResponses[i].PublicKey)
		log.Debug("append signature (non-segwit)")
		// if sigScript != nil {
		// 	log.Debug("append sigScript (non-segwit)")
		// 	builder.AddData(sigScript)
		// }
		txObj.MsgTx.TxIn[i].SignatureScript, err = builder.Script()
		if err != nil {
			return err
		}
	}

	txObj.Signed = true
	return nil
}

func (tx *Tx) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := tx.MsgTx.Serialize(buf); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}
