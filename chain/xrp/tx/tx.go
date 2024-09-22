package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	binarycodec "github.com/xyield/xrpl-go/binary-codec"

	xc "github.com/cordialsys/crosschain"
)

const (
	PAYMENT TransactionType = "Payment"
)

type TransactionType string

type XRPTransaction struct {
	Account     xc.Address          `json:"Account,omitempty"`     // Sending account address
	Amount      xc.AmountBlockchain `json:"Amount,omitempty"`      // Amount to deliver
	Destination xc.Address          `json:"Destination,omitempty"` // Destination account
	Fee         string              `json:"Fee,omitempty"`         // Optional fee (if none, XRPL calculates it)
	Flags       int                 `json:"Flags"`                 // Flags for this transaction
	//LastLedgerSequence int                 `json:"LastLedgerSequence"`        // Optional last ledger sequence
	Sequence        int             `json:"Sequence,omitempty"`        // Sequence number of the account
	SigningPubKey   string          `json:"SigningPubKey,omitempty"`   // Public key of the account
	TransactionType TransactionType `json:"TransactionType,omitempty"` // Type of the transaction
	TxnSignature    string          `json:"TxnSignature,omitempty"`    // Transaction signature
	//AccountTxnID       string              `json:"AccountTxnID"`       // Hash of a previous transaction
	//DeliverMin     *Amount `json:"DeliverMin,omitempty"`     // Optional minimum amount to be delivered
	//DestinationTag *uint32 `json:"DestinationTag,omitempty"` // Optional destination tag
	//InvoiceID      *string `json:"InvoiceID,omitempty"`      // Optional invoice ID (256-bit hash)
	//Memos              *[]Memo         `json:"memos,omitempty"`              // Optional memos
	//NetworkID      *uint32 `json:"NetworkID,omitempty"`      // Optional network ID
	//Paths   *[]Path `json:"Paths,omitempty"`   // Optional paths for cross-currency payments
	//SendMax *Amount `json:"SendMax,omitempty"` // Optional maximum amount allowed to be spent
	//Signers        *[]Signer `json:"signers,omitempty"`         // Optional signers for multisig
	//SourceTag      *uint32 `json:"SourceTag,omitempty"`      // Optional source tag
	//TicketSequence *uint32 `json:"TicketSequence,omitempty"` // Optional ticket sequence
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
	return xc.TxHash("not implemented")
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	if tx.XRPTx == nil {
		return nil, errors.New("missing XRP transaction")
	}

	if tx.EncodeForSigning == nil {
		return nil, errors.New("missing serialised XRP transaction")
	}

	hash := sha256.Sum256(tx.EncodeForSigning)

	return []xc.TxDataToSign{hash[:]}, nil

}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	for _, sig := range signatures {
		signatureHex := hex.EncodeToString(sig)
		tx.XRPTx.TxnSignature = signatureHex
		tx.TransactionSignature = append(tx.TransactionSignature, sig)
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

	result := make(map[string]interface{})
	result["Account"] = string(xrpTx.Account)
	result["Amount"] = xrpTx.Amount.String()
	result["Destination"] = string(xrpTx.Destination)
	result["Fee"] = xrpTx.Fee
	result["Flags"] = xrpTx.Flags
	//result["LastLedgerSequence"] = xrpTx.LastLedgerSequence
	result["Sequence"] = xrpTx.Sequence
	result["SigningPubKey"] = xrpTx.SigningPubKey
	result["TransactionType"] = string(xrpTx.TransactionType)
	result["TxnSignature"] = xrpTx.TxnSignature

	encodeTx, err := binarycodec.Encode(result)
	if err != nil {
		return nil, fmt.Errorf("failed to serialise serialised XRP transaction: %v", err)
	}

	// TODO: remove, only for test.
	decode, err := binarycodec.Decode(encodeTx)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("decode:", decode)

	return []byte(encodeTx), nil
}
