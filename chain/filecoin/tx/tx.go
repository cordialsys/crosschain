package tx

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/filecoin/address"
	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	"github.com/fxamacker/cbor"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

const txHashSize = 32

// Transaction hash and signature CID prefix
var cidPrefix = []byte{0x01, 0x71, 0xa0, 0xe4, 0x02, 0x20}

type Message struct {
	Version    uint64
	To         string
	From       string
	Nonce      uint64
	Value      xc.AmountBlockchain
	GasLimit   uint64
	GasFeeCap  xc.AmountBlockchain
	GasPremium xc.AmountBlockchain
	Method     uint64
	Params     []byte
}

func NewMessage(args xcbuilder.TransferArgs, txInput tx_input.TxInput) Message {
	to := string(args.GetTo())
	from := string(args.GetFrom())
	amount := args.GetAmount()
	return Message{
		Version:    0,
		To:         to,
		From:       from,
		Nonce:      txInput.Nonce,
		Value:      xc.NewAmountBlockchainFromUint64(amount.Int().Uint64()),
		GasLimit:   txInput.GasLimit,
		GasFeeCap:  txInput.GasFeeCap,
		GasPremium: txInput.GasPremium,
		Method:     0,
		Params:     []byte{},
	}
}

// Filecoin signature type.
type Signature struct {
	Type byte
	Data []byte
}

// Filecoint transaction
type Tx struct {
	Message      Message
	XcSignatures []xc.TxSignature
	Signature    Signature
}

type SignedTx struct {
	Tx        *Tx
	Signature Signature
}

var _ xc.Tx = &Tx{}

// Return hash of Filecoin transaction
// Works only for signed transactions
func (tx Tx) Hash() xc.TxHash {
	bytes, err := tx.SerializeSigned()
	if err != nil {
		logrus.Errorf("failed to serialize signed tx: %v", err)
		return xc.TxHash("")
	}

	h, err := blake2b.New(txHashSize, nil)
	if err != nil {
		logrus.Errorf("failed to create blake2b hasher: %v", err)
		return xc.TxHash("")
	}

	h.Write(bytes)
	sum := h.Sum(nil)
	cid := append(cidPrefix, sum...)

	return xc.TxHash("b" + address.Encoding.WithPadding(-1).EncodeToString(cid))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	bytes, err := tx.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx: %v", err)
	}

	h, err := blake2b.New(txHashSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blake2b hasher: %v", err)
	}

	h.Write(bytes)
	sum := h.Sum(nil)
	cid := append(cidPrefix, sum...)

	h, err = blake2b.New(txHashSize, nil)
	h.Write(cid)
	sum = h.Sum(nil)
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sum)}, err
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != 1 {
		return errors.New("only one signature is allowed")
	}

	if len(tx.XcSignatures) > 0 || tx.Signature.Data != nil {
		return errors.New("transaction already signed")
	}
	tx.XcSignatures = []xc.TxSignature{signatures[0].Signature}

	signature := signatures[0]
	tx.Signature = Signature{
		Type: address.ProtocolSecp256k1,
		Data: signature.Signature,
	}

	return nil
}

// Serialize filecoin transaction using CBOR
func (tx Tx) Serialize() ([]byte, error) {
	to, err := address.AddressToBytes(tx.Message.To)
	if err != nil {
		return nil, fmt.Errorf("invalid `to` address: %w", err)
	}

	from, err := address.AddressToBytes(tx.Message.From)
	if err != nil {
		return nil, fmt.Errorf("invalid `from` address: %w", err)
	}

	i := []interface{}{
		tx.Message.Version,
		to,
		from,
		tx.Message.Nonce,
		append([]byte{0}, tx.Message.Value.Bytes()...),
		tx.Message.GasLimit,
		append([]byte{0}, tx.Message.GasFeeCap.Bytes()...),
		append([]byte{0}, tx.Message.GasPremium.Bytes()...),
		tx.Message.Method,
		append([]byte{}, tx.Message.Params...),
	}

	return cbor.Marshal(i, cbor.CanonicalEncOptions())
}

// Serialize signed filecoin transaction using CBOR
func (tx Tx) SerializeSigned() ([]byte, error) {
	if len(tx.Signature.Data) == 0 {
		return nil, errors.New("signature is missing")
	}

	to, err := address.AddressToBytes(tx.Message.To)
	if err != nil {
		return nil, fmt.Errorf("invalid `to` address: %w", err)
	}

	from, err := address.AddressToBytes(tx.Message.From)
	if err != nil {
		return nil, fmt.Errorf("invalid `from` address: %w", err)
	}

	i := []interface{}{
		[]interface{}{
			tx.Message.Version,
			to,
			from,
			tx.Message.Nonce,
			append([]byte{0}, tx.Message.Value.Bytes()...),
			tx.Message.GasLimit,
			append([]byte{0}, tx.Message.GasFeeCap.Bytes()...),
			append([]byte{0}, tx.Message.GasPremium.Bytes()...),
			tx.Message.Method,
			append([]byte{}, tx.Message.Params...),
		},
		append([]byte{address.ProtocolSecp256k1}, tx.Signature.Data...),
	}

	return cbor.Marshal(i, cbor.CanonicalEncOptions())
}
