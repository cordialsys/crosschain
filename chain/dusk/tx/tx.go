package tx

import (
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/cloudflare/circl/sign/bls"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/dusk"
	"github.com/cordialsys/crosschain/chain/dusk/address"
	"github.com/cordialsys/crosschain/chain/dusk/tx_input"
)

type PayloadType uint8

const (
	// https://github.com/dusk-network/rusk/blob/master/core/src/transfer/moonlight.rs#L402-L417
	PayloadTypeCall   PayloadType = 1
	PayloadTypeDeploy PayloadType = 2
	PayloadTypeMemo   PayloadType = 3
)

// Tx for Dusk
type Tx struct {
	Payload   Payload
	Signature []byte
}

type Fee struct {
	GasLimit      uint64
	GasPrice      uint64
	RefundAddress bls.PublicKey[bls.G2]
}

type Payload struct {
	// ID of the chain for this transaction to execute on.
	ChainId uint8
	// Key of the sender of this transaction.
	Sender bls.PublicKey[bls.G2]
	// Key of the receiver of the funds.
	Receiver bls.PublicKey[bls.G2]
	// Value to be transferred.
	Value uint64
	// Deposit for a contract.
	Deposit uint64
	// Data used to calculate the transaction fee and refund unspent gas.
	Fee Fee
	// Nonce used for replay protection. Moonlight nonces are strictly
	// increasing and incremental, meaning that for a transaction to be
	// valid, only the current nonce + 1 can be used.
	//
	// The current nonce is queryable via the transfer contract or account
	// status.
	Nonce uint64
	Memo  []byte
}

func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (Tx, error) {
	senderKey, err := address.GetPublicKeyFromAddress(args.GetFrom())
	if err != nil {
		return Tx{}, errors.New("failed to get public key from sender address")
	}

	receiverKey, err := address.GetPublicKeyFromAddress(args.GetTo())
	if err != nil {
		return Tx{}, errors.New("failed to get public key from receiver address")
	}

	refoundKey, err := address.GetPublicKeyFromAddress(input.RefundAccount)
	if err != nil {
		return Tx{}, errors.New("failed to get public key from refund address")
	}
	memo := ""
	if inputMemo, ok := args.GetMemo(); ok {
		memo = inputMemo
	}

	tx := Tx{
		Payload: Payload{
			ChainId:  input.ChainId,
			Sender:   senderKey,
			Receiver: receiverKey,
			Value:    args.GetAmount().Int().Uint64(),
			Deposit:  0,
			Fee: Fee{
				GasLimit:      input.GasLimit,
				GasPrice:      input.GasPrice,
				RefundAddress: refoundKey,
			},
			Nonce: input.Nonce,
			Memo:  []byte(memo),
		},
	}

	return tx, nil
}

var _ xc.Tx = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	sighashes, err := tx.Sighashes()
	if err != nil {
		return xc.TxHash("failed to get sighashes")
	}
	if len(sighashes) != 1 {
		return xc.TxHash("expected only one sighash")
	}

	sighash := sighashes[0]
	sighash = append(sighash, tx.Signature...)
	scalar, err := dusk.Blake2bScalarReduce(sighash)
	if err != nil {
		return xc.TxHash("failed to get scalar from sighash")
	}

	bytes, err := dusk.ScalarToLeBytes(scalar)
	if err != nil {
		return xc.TxHash("failed to marshal scalar")
	}

	hash := hex.EncodeToString(bytes)
	return xc.TxHash(hash)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	bytes := make([]byte, 0)
	bytes = append(bytes, tx.Payload.ChainId)

	senderBytes, err := tx.Payload.Sender.MarshalBinary()
	if err != nil {
		return []xc.TxDataToSign{}, errors.New("failed to marshal sender")
	}
	bytes = append(bytes, senderBytes...)
	if tx.Payload.Receiver != tx.Payload.Sender {
		receiverBytes, err := tx.Payload.Receiver.MarshalBinary()
		if err != nil {
			return []xc.TxDataToSign{}, errors.New("failed to marshal receiver")
		}
		bytes = append(bytes, receiverBytes...)
	}

	bytes = binary.LittleEndian.AppendUint64(bytes, tx.Payload.Value)
	bytes = binary.LittleEndian.AppendUint64(bytes, tx.Payload.Deposit)
	bytes = binary.LittleEndian.AppendUint64(bytes, tx.Payload.Fee.GasLimit)
	bytes = binary.LittleEndian.AppendUint64(bytes, tx.Payload.Fee.GasPrice)
	if tx.Payload.Fee.RefundAddress != tx.Payload.Sender {
		refundAddressBytes, err := tx.Payload.Fee.RefundAddress.MarshalBinary()
		if err != nil {
			return []xc.TxDataToSign{}, errors.New("failed to marshal refund address")
		}
		bytes = append(bytes, refundAddressBytes...)
	}
	bytes = binary.LittleEndian.AppendUint64(bytes, tx.Payload.Nonce)

	if len(tx.Payload.Memo) > 0 {
		// Note that this is different from how it's serialized
		// https://github.com/dusk-network/rusk/blob/master/core/src/transfer/moonlight.rs#L516-L532
		bytes = append(bytes, tx.Payload.Memo...)
	}

	return []xc.TxDataToSign{bytes}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if len(signatures) != 1 {
		return errors.New("only one signature is allowed")
	}

	if len(tx.Signature) > 0 {
		return errors.New("transaction already signed")
	}

	signature := signatures[0]
	tx.Signature = signature

	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return []xc.TxSignature{xc.TxSignature(tx.Signature)}
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	payload := make([]byte, 0)

	payload = append(payload, tx.Payload.ChainId)
	senderBytes, err := tx.Payload.Sender.MarshalBinary()
	if err != nil {
		return []byte{}, errors.New("failed to marshal sender")
	}
	payload = append(payload, senderBytes...)

	if tx.Payload.Sender == tx.Payload.Receiver {
		payload = append(payload, 0)
	} else {
		payload = append(payload, 1)
		receiverBytes, err := tx.Payload.Receiver.MarshalBinary()
		if err != nil {
			return []byte{}, errors.New("failed to marshal receiver")
		}
		payload = append(payload, receiverBytes...)
	}

	payload = binary.LittleEndian.AppendUint64(payload, tx.Payload.Value)
	payload = binary.LittleEndian.AppendUint64(payload, tx.Payload.Deposit)
	payload = binary.LittleEndian.AppendUint64(payload, tx.Payload.Fee.GasLimit)
	payload = binary.LittleEndian.AppendUint64(payload, tx.Payload.Fee.GasPrice)

	if tx.Payload.Fee.RefundAddress == tx.Payload.Sender {
		payload = append(payload, 0)
	} else {
		payload = append(payload, 1)
		refundAddressBytes, err := tx.Payload.Fee.RefundAddress.MarshalBinary()
		if err != nil {
			return []byte{}, errors.New("failed to marshal refund address")
		}
		payload = append(payload, refundAddressBytes...)
	}

	payload = binary.LittleEndian.AppendUint64(payload, tx.Payload.Nonce)
	if len(tx.Payload.Memo) > 0 {
		// https://github.com/dusk-network/rusk/blob/master/core/src/transfer/moonlight.rs#L402-L417
		payload = append(payload, 3)
		payload = binary.LittleEndian.AppendUint64(payload, uint64(len(tx.Payload.Memo)))
		payload = append(payload, tx.Payload.Memo...)
	} else {
		payload = append(payload, 0)
	}

	bytes := make([]byte, 0)
	// Add 1 for MoonlightTransaction
	bytes = append(bytes, 1)
	bytes = binary.LittleEndian.AppendUint64(bytes, uint64(len(payload)))
	bytes = append(bytes, payload...)
	bytes = append(bytes, tx.Signature...)
	return bytes, nil
}
