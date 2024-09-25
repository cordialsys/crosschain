package tx

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	xc "github.com/cordialsys/crosschain"
	binarycodec "github.com/xyield/xrpl-go/binary-codec"
	"math/big"
	"strings"
)

const (
	PAYMENT                 TransactionType = "Payment"
	TRANSACTION_HASH_PREFIX                 = "54584E00"
)

type TransactionType string

type XRPTransaction struct {
	Account            xc.Address       `json:"Account"`
	Amount             AmountBlockchain `json:"Amount"`
	Destination        xc.Address       `json:"Destination"`
	Fee                string           `json:"Fee"`
	Flags              int64            `json:"Flags,omitempty"`
	LastLedgerSequence int64            `json:"LastLedgerSequence"`
	Sequence           int64            `json:"Sequence"`
	SigningPubKey      string           `json:"SigningPubKey"`
	TransactionType    TransactionType  `json:"TransactionType"`
	TxnSignature       string           `json:"TxnSignature"`
}

type AmountBlockchain struct {
	StringValue string  `json:"-"`
	AmountValue *Amount `json:"-"`
	IsString    bool    `json:"-"`
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
	EncodeForSigning     []byte
	EncodeXRPTx          *string
	TransactionSignature []xc.TxSignature
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	serializedTxInput, err := tx.Serialize()
	if err != nil {
		return xc.TxHash("")
	}

	encodeTxWithPrefix := TRANSACTION_HASH_PREFIX + string(serializedTxInput)

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

	if tx.EncodeForSigning == nil {
		return nil, errors.New("missing serialised XRP transaction")
	}

	// For k256 signing, XRP uses sha512[:32]
	// https://github.com/XRPLF/xrpl-py/blob/17aad31f77452d30917b9e4544c9c87c274c0e3d/xrpl/core/keypairs/secp256k1.py#L95
	digestSha512 := sha512.Sum512(tx.EncodeForSigning)
	firstHalf := digestSha512[:32]

	return []xc.TxDataToSign{firstHalf[:]}, nil

}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.TransactionSignature != nil {
		return errors.New("transaction already signed")
	}

	for _, rsvBytes := range signatures {
		r, s, err := DecodeEcdsaSignature(rsvBytes)
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

	amountLargeNumber := new(big.Int)
	amountLargeNumber.SetString(xrpTx.Amount.StringValue, 10)

	divisor := big.NewInt(1000)

	XRPAmount := new(big.Int)
	XRPAmount.Div(amountLargeNumber, divisor)

	result := make(map[string]interface{})
	result["Account"] = string(xrpTx.Account)
	result["Amount"] = XRPAmount.String()
	result["Destination"] = string(xrpTx.Destination)
	result["Fee"] = xrpTx.Fee
	result["Flags"] = xrpTx.Flags
	result["LastLedgerSequence"] = xrpTx.LastLedgerSequence
	result["Sequence"] = xrpTx.Sequence
	result["SigningPubKey"] = xrpTx.SigningPubKey
	result["TransactionType"] = string(xrpTx.TransactionType)
	result["TxnSignature"] = xrpTx.TxnSignature

	encodeTx, err := binarycodec.Encode(result)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to serialise serialised XRP transaction: %v", err)
	}

	encodeTxWithPrefix := TRANSACTION_HASH_PREFIX + string(encodeTx)

	decodedBytes, err := hex.DecodeString(encodeTxWithPrefix)

	hash := sha512.Sum512(decodedBytes)

	firstHalf := hash[:32]
	hashHex := hex.EncodeToString(firstHalf)
	fmt.Println(hashHex)

	return []byte(encodeTx), nil
}

// ExtractAssetAndContract parse assetContract and returns asset and contract
func ExtractAssetAndContract(assetContract string) (string, string, error) {
	var separator string

	switch {
	case strings.Contains(assetContract, "."):
		separator = "."
	case strings.Contains(assetContract, "-"):
		separator = "-"
	case strings.Contains(assetContract, "_"):
		separator = "_"
	default:
		return "", "", fmt.Errorf("string must contain one of the following separators: '.', '-', '_'")
	}

	parts := strings.Split(assetContract, separator)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format, string should contain exactly one separator")
	}

	asset := parts[0]
	contract := parts[1]

	return asset, contract, nil
}
