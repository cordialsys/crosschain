package tx

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/near/tx_input"
	bin "github.com/gagliardetto/binary"
)

const (
	ActionFunctionCall   = 2
	ActionTypeTransfer   = 3
	BlockHashLen         = 32
	KeyAmount            = "amount"
	KeyReceiverId        = "receiver_id"
	MethodNameFtTransfer = "ft_transfer"
	PublicKeyLen         = 32
	SignatureLen         = 64
)

type Tx[T any] struct {
	Transaction Transaction[T]
	Signature   Signature
}

type TransferAction struct {
	Type   uint8
	Action Uint128
}

func NewTransferAction(amount Uint128) TransferAction {
	return TransferAction{
		Type:   ActionTypeTransfer,
		Action: amount,
	}
}

type FunctionCallAction struct {
	Type       uint8
	MethodName string
	Args       []byte
	Gas        uint64
	Deposit    [16]byte
}

func NewFunctionCallAction(contract string, receiver string, amount xc.AmountBlockchain, deposit xc.AmountBlockchain) (FunctionCallAction, error) {
	args, err := json.Marshal(map[string]any{
		KeyReceiverId: receiver,
		KeyAmount:     amount.ToHuman(0).String(),
	})
	if err != nil {
		return FunctionCallAction{}, fmt.Errorf("failed to marshal function call args: %w", err)
	}

	d := xc.NewAmountBlockchainFromUint64(1)
	depositUint128, err := Uint128FromAmountBlockchain(d)
	if err != nil {
		return FunctionCallAction{}, fmt.Errorf("failed to convert deposit amount to uint128: %w", err)
	}
	depositAmountBz, err := bin.MarshalBorsh(depositUint128)
	if err != nil {
		return FunctionCallAction{}, fmt.Errorf("failed to marshal deposit amount: %w", err)
	}
	if len(depositAmountBz) != 16 {
		return FunctionCallAction{}, fmt.Errorf("invalid deposit amount length")
	}

	var depositAmountBytes [16]byte
	copy(depositAmountBytes[:], depositAmountBz)
	return FunctionCallAction{
		Type:       ActionFunctionCall,
		MethodName: MethodNameFtTransfer,
		Args:       args,
		Deposit:    depositAmountBytes,
	}, nil
}

// PublicKey represents a NEAR public key
type PublicKey struct {
	KeyType uint8    // 0 for ED25519
	Data    [32]byte // 32 bytes for ED25519
}

// Signature represents a NEAR public key
type Signature struct {
	KeyType uint8    // 0 for ED25519
	Data    [64]byte // 32 bytes for ED25519
}

type Uint128 struct {
	Lo uint64
	Hi uint64
}

func Uint128FromAmountBlockchain(amount xc.AmountBlockchain) (Uint128, error) {
	if amount.Sign() < 0 {
		return Uint128{}, fmt.Errorf("negative value")
	}
	if amount.Int().BitLen() > 128 {
		return Uint128{}, fmt.Errorf("overflow uint128")
	}

	lo := amount.Uint64()
	hi := new(big.Int).Rsh(amount.Int(), 64).Uint64()

	return Uint128{
		Lo: lo,
		Hi: hi,
	}, nil
}

// Transfer action
type Transaction[T any] struct {
	SignerID   string
	PublicKey  PublicKey
	Nonce      uint64
	ReceiverID string
	BlockHash  [32]byte
	Actions    []T
}

var _ xc.Tx = &Tx[TransferAction]{}

func NewNativeTx(input *tx_input.TxInput, args xcbuilder.TransferArgs) (*Tx[TransferAction], error) {
	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()
	txAmount, err := Uint128FromAmountBlockchain(amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to uint128: %w", err)
	}
	transfer := NewTransferAction(txAmount)

	publicKey, ok := args.GetPublicKey()
	if !ok {
		return nil, errors.New("public key is required for NEAR transactions")
	}
	if len(publicKey) != PublicKeyLen {
		return nil, fmt.Errorf("public key len should be exactly %d, got %d", PublicKeyLen, len(publicKey))
	}
	var pkbz [PublicKeyLen]byte
	copy(pkbz[:], publicKey)

	var blockHash [BlockHashLen]byte
	blockHashBytes := base58.Decode(input.BlockHash)
	copy(blockHash[:], blockHashBytes)
	return &Tx[TransferAction]{
		Transaction: Transaction[TransferAction]{
			SignerID: string(from),
			PublicKey: PublicKey{
				KeyType: 0,
				Data:    pkbz,
			},
			Nonce:      input.Nonce,
			ReceiverID: string(to),
			BlockHash:  blockHash,
			Actions: []TransferAction{
				transfer,
			},
		},
	}, nil
}

func NewTokenTx(input *tx_input.TxInput, args xcbuilder.TransferArgs) (*Tx[FunctionCallAction], error) {
	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()
	contract, ok := args.GetContract()
	if !ok {
		return nil, fmt.Errorf("contract is reqauired for token transfers")
	}
	tokenTransfer, err := NewFunctionCallAction(string(contract), string(to), amount, input.RequiredDepopsit)
	if err != nil {
		return nil, fmt.Errorf("failed to create funcion call action: %w", err)
	}
	tokenTransfer.Gas = input.GasCost.Uint64()

	publicKey, ok := args.GetPublicKey()
	if !ok {
		return nil, errors.New("public key is required for NEAR transactions")
	}
	if len(publicKey) != PublicKeyLen {
		return nil, fmt.Errorf("public key len should be exactly %d, got %d", PublicKeyLen, len(publicKey))
	}
	var pkbz [PublicKeyLen]byte
	copy(pkbz[:], publicKey)

	var blockHash [BlockHashLen]byte
	blockHashBytes := base58.Decode(input.BlockHash)
	copy(blockHash[:], blockHashBytes)
	return &Tx[FunctionCallAction]{
		Transaction: Transaction[FunctionCallAction]{
			SignerID: string(from),
			PublicKey: PublicKey{
				KeyType: 0,
				Data:    pkbz,
			},
			Nonce:      input.Nonce,
			ReceiverID: string(contract),
			BlockHash:  blockHash,
			Actions: []FunctionCallAction{
				tokenTransfer,
			},
		},
	}, nil
}

// Hash returns the tx hash or id
func (tx Tx[T]) Hash() xc.TxHash {
	bz, err := bin.MarshalBorsh(tx.Transaction)
	if err != nil {
		return xc.TxHash("")
	}
	hash := sha256.Sum256(bz)
	return xc.TxHash(base58.Encode(hash[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx[T]) Sighashes() ([]*xc.SignatureRequest, error) {
	bz, err := bin.MarshalBorsh(tx.Transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx: %w", err)
	}

	hash := sha256.Sum256(bz)
	return []*xc.SignatureRequest{
		{
			Payload: hash[:],
		},
	}, nil

}

// SetSignatures adds a signature to Tx
func (tx *Tx[T]) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != 1 {
		return errors.New("expected exactly 1 signature")
	}
	signature := signatures[0]
	if len(signature.Signature) != SignatureLen {
		return fmt.Errorf("expected signature len is %d, got %d", SignatureLen, len(signature.Signature))
	}

	var signatureData [64]byte
	copy(signatureData[:], signature.Signature)
	tx.Signature = Signature{
		KeyType: 0,
		Data:    signatureData,
	}

	return nil
}

// Serialize returns the serialized tx
func (tx Tx[T]) Serialize() ([]byte, error) {

	return bin.MarshalBorsh(tx)
}
