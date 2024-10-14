package tx

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	xc "github.com/cordialsys/crosschain"
	btctx "github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/sirupsen/logrus"
	binarycodec "github.com/xyield/xrpl-go/binary-codec"
)

const (
	PAYMENT                 TransactionType = "Payment"
	TRANSACTION_HASH_PREFIX                 = "54584E00"
)

type TransactionType string

type XRPTransaction struct {
	Account            xc.Address       `json:"Account"`
	Amount             AmountBlockchain `json:"Amount"`
	SendMax            Amount           `json:"SendMax"`
	Destination        xc.Address       `json:"Destination"`
	DestinationTag     int64            `json:"DestinationTag"`
	Fee                string           `json:"Fee"`
	Flags              int64            `json:"Flags,omitempty"`
	LastLedgerSequence int64            `json:"LastLedgerSequence"`
	Sequence           int64            `json:"Sequence"`
	SigningPubKey      string           `json:"SigningPubKey"`
	TransactionType    TransactionType  `json:"TransactionType"`
	TxnSignature       string           `json:"TxnSignature"`
}

type AmountBlockchain struct {
	XRPAmount   string  `json:"-"`
	TokenAmount *Amount `json:"-"`
}

type Amount struct {
	Currency string `json:"currency"`
	Issuer   string `json:"issuer"`
	Value    string `json:"value"`
}

// Tx for Template
type Tx struct {
	XRPTx                *XRPTransaction
	SignPubKey           []byte
	TransactionSignature []xc.TxSignature
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	serializedTxInputBytes, err := tx.Serialize()
	if err != nil {
		return xc.TxHash("")
	}

	serializedTxInputHex := hex.EncodeToString(serializedTxInputBytes)

	encodeTxWithPrefix := TRANSACTION_HASH_PREFIX + string(serializedTxInputHex)

	decodedBytes, err := hex.DecodeString(encodeTxWithPrefix)
	if err != nil {
		return xc.TxHash("")
	}

	hash := sha512.Sum512(decodedBytes)
	firstHalf := hash[:32]
	hashHex := hex.EncodeToString(firstHalf)

	return xc.TxHash(hashHex)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	if tx.XRPTx == nil {
		return nil, errors.New("missing XRP transaction")
	}

	resultMapXRP, renderErr := RenderToMap(*tx.XRPTx)
	if renderErr != nil {
		return nil, fmt.Errorf("error rendering transaction to map: %v", renderErr)
	}

	encodeForSigningHex, err := binarycodec.EncodeForSigning(resultMapXRP)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction for signing %v", err)
	}

	encodeForSigningBytes, err := hex.DecodeString(encodeForSigningHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create byte object from hex serialized transaction %v", err)
	}

	// For k256 signing, XRP uses sha512[:32]
	// https://github.com/XRPLF/xrpl-py/blob/17aad31f77452d30917b9e4544c9c87c274c0e3d/xrpl/core/keypairs/secp256k1.py#L95
	digestSha512 := sha512.Sum512(encodeForSigningBytes)
	firstHalf := digestSha512[:32]

	return []xc.TxDataToSign{firstHalf[:]}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.TransactionSignature != nil {
		return errors.New("transaction already signed")
	}

	for _, rsvBytes := range signatures {
		r, s, err := btctx.DecodeEcdsaSignature(rsvBytes)
		if err != nil {
			return err
		}

		signature := ecdsa.NewSignature(&r, &s)
		signatureBytes := signature.Serialize()
		signatureHex := hex.EncodeToString(signatureBytes)
		tx.XRPTx.TxnSignature = signatureHex
		tx.TransactionSignature = append(tx.TransactionSignature, rsvBytes)
	}

	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.TransactionSignature
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {

	if tx.XRPTx == nil {
		return []byte{}, errors.New("missing XRP transaction")
	}

	xrpTx := tx.XRPTx

	if xrpTx.LastLedgerSequence == 0 {
		return []byte{}, errors.New("missing last ledger sequence")
	}

	if xrpTx.TxnSignature == "" {
		return []byte{}, errors.New("missing transaction signature")
	}

	resultMapXRP, renderErr := RenderToMap(*xrpTx)
	if renderErr != nil {
		return []byte{}, fmt.Errorf("failed to render XRP transaction: %w", renderErr)
	}

	encodedTx, err := binarycodec.Encode(resultMapXRP)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to serialise serialised XRP transaction: %v", err)
	}

	decodedBytes, err := hex.DecodeString(encodedTx)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to decode XRP transaction: %v", err)
	}
	serializedTxInputHex := hex.EncodeToString(decodedBytes)
	fmt.Println(serializedTxInputHex)

	hashHex, err := HashFromTx(encodedTx)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate transaction hash")
	}
	logrus.WithField("hash", hashHex).Debug("Transaction hash")

	return decodedBytes, nil
}

func HashFromTx(encodeTx string) (string, error) {
	encodeTxWithPrefix := TRANSACTION_HASH_PREFIX + string(encodeTx)

	decodedBytes, err := hex.DecodeString(encodeTxWithPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to decode XRP transaction: %w", err)
	}

	hash := sha512.Sum512(decodedBytes)
	firstHalf := hash[:32]
	hashHex := hex.EncodeToString(firstHalf)

	return hashHex, nil
}

func RenderToMap(xrpTx XRPTransaction) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	result["Account"] = string(xrpTx.Account)
	result["Destination"] = string(xrpTx.Destination)
	result["DestinationTag"] = int(xrpTx.DestinationTag)
	result["Fee"] = xrpTx.Fee
	result["Flags"] = int(xrpTx.Flags)
	result["LastLedgerSequence"] = int(xrpTx.LastLedgerSequence)
	result["Sequence"] = int(xrpTx.Sequence)
	result["SigningPubKey"] = xrpTx.SigningPubKey
	result["TransactionType"] = string(xrpTx.TransactionType)
	result["TxnSignature"] = xrpTx.TxnSignature

	if xrpTx.Amount.XRPAmount != "" {
		amountRenderErr := RenderXrpAmount(result, xrpTx.Amount.XRPAmount)
		if amountRenderErr != nil {
			return nil, fmt.Errorf("failed to render XRP amount: %w", amountRenderErr)
		}
	} else {
		RenderTokenAmount(result, xrpTx.Amount.TokenAmount)
		RenderSendMax(result, &xrpTx.SendMax)
	}

	return result, nil
}

func RenderXrpAmount(fields map[string]interface{}, amount string) error {
	fields["Amount"] = amount

	return nil
}

func RenderTokenAmount(fields map[string]interface{}, amount *Amount) {
	fields["Amount"] = map[string]interface{}{
		"currency": amount.Currency,
		"issuer":   amount.Issuer,
		"value":    amount.Value,
	}
}

func RenderSendMax(fields map[string]interface{}, sendMax *Amount) {
	fields["SendMax"] = map[string]interface{}{
		"currency": sendMax.Currency,
		"issuer":   sendMax.Issuer,
		"value":    sendMax.Value,
	}
}
