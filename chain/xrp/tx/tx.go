package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	Account            xc.Address          `json:"Account,omitempty"`         // Sending account address
	Amount             xc.AmountBlockchain `json:"Amount,omitempty"`          // Amount to deliver
	Destination        xc.Address          `json:"Destination,omitempty"`     // Destination account
	Fee                string              `json:"Fee,omitempty"`             // Optional fee (if none, XRPL calculates it)
	Flags              int                 `json:"Flags"`                     // Flags for this transaction
	LastLedgerSequence int                 `json:"LastLedgerSequence"`        // Optional last ledger sequence
	Sequence           int                 `json:"Sequence,omitempty"`        // Sequence number of the account
	SigningPubKey      string              `json:"SigningPubKey,omitempty"`   // Public key of the account
	TransactionType    TransactionType     `json:"TransactionType,omitempty"` // Type of the transaction
	TxnSignature       string              `json:"TxnSignature,omitempty"`    // Transaction signature
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
	XRPTx             *XRPTransaction
	SignPubKey        *string
	SerialisedXRPTx   *string
	SerialisedForSign *string
	InputSignature    []xc.TxSignature
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

	if tx.SerialisedForSign == nil {
		return nil, errors.New("missing serialised XRP transaction")
	}

	hash := sha256.Sum256([]byte(*tx.SerialisedForSign))

	return []xc.TxDataToSign{hash[:]}, nil

}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	for _, sig := range signatures {
		signatureHex := hex.EncodeToString(sig)
		tx.XRPTx.TxnSignature = signatureHex
		tx.InputSignature = append(tx.InputSignature, sig)
	}

	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.InputSignature
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	if tx.XRPTx == nil {
		return []byte{}, errors.New("missing XRP transaction")
	}

	//tx.XRPTx.SigningPubKey = hex.EncodeToString(txInput.Pubkey) // TODO: Possible this need to be added after signing it.
	tx.XRPTx.SigningPubKey = *tx.SignPubKey

	jsonData, err := json.Marshal(tx.XRPTx)
	if err != nil {
		return nil, errors.New("error marshalling struct")
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, errors.New("error unmarshalling struct")
	}

	if sequence, ok := result["Sequence"].(float64); ok {
		result["Sequence"] = int(sequence)
	}

	if flags, ok := result["Flags"].(float64); ok {
		result["Flags"] = int(flags)
	}

	if lastLedgerIndex, ok := result["LastLedgerSequence"].(float64); ok {
		result["LastLedgerSequence"] = int(lastLedgerIndex)
	}

	if txnSignature, ok := result["LastLedgerSequence"].(string); ok {
		result["LastLedgerSequence"] = []byte(txnSignature)
	}

	serializedForSigning, err := binarycodec.Encode(result)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("serializedForSigning:", serializedForSigning)

	decode, err := binarycodec.Decode(serializedForSigning)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("decode:", decode)

	return []byte(serializedForSigning), nil
}
