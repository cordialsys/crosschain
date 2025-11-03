package tx

import (
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	commontypes "github.com/cordialsys/crosschain/chain/hedera/common_types"
	"github.com/cordialsys/crosschain/chain/hedera/tx_input"
	"github.com/cordialsys/hedera-protobufs-go/common"
	"github.com/cordialsys/hedera-protobufs-go/services"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Tx for Hedera
type Tx struct {
	TxBody   *services.TransactionBody
	SignedTx *services.SignedTransaction
}

var _ xc.Tx = &Tx{}

type ProcessedInput struct {
	AccountId     *common.AccountID
	MaxFee        uint64
	Memo          string
	NodeId        *common.AccountID
	TransactionId *common.TransactionID
	ValidTime     int64
}

func validateInput(input xc.TxInput) (*ProcessedInput, error) {
	txi, ok := input.(*tx_input.TxInput)
	if !ok {
		return nil, errors.New("invalid input type")
	}

	if len(txi.Memo) > commontypes.MAX_MEMO_LENGTH {
		return nil, errors.New("invalid memo")
	}

	accId, err := commontypes.NewAccountId(txi.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to create account id: %w", err)
	}
	unixTimestamp := time.Unix(0, txi.ValidStartTimestamp)
	txId, err := commontypes.NewTransactionId(txi.AccountId, unixTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction id: %w", err)
	}

	nodeId, err := commontypes.NewHederaAccountId(txi.NodeAccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to create node id: %w", err)
	}

	return &ProcessedInput{
		AccountId:     accId,
		MaxFee:        txi.MaxTransactionFee,
		Memo:          txi.Memo,
		NodeId:        nodeId,
		TransactionId: txId,
		ValidTime:     txi.ValidTime,
	}, nil
}

func NewTransfer(args builder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	txi, err := validateInput(input)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	destinationId, err := commontypes.NewAccountId(string(args.GetTo()))
	if err != nil {
		return nil, fmt.Errorf("failed to convert desination address: %w", err)
	}

	// prepare transfer amounts
	amount := args.GetAmount()
	accountAmounts := []*common.AccountAmount{
		{
			AccountID:  txi.AccountId,
			Amount:     int64(amount.Uint64()) * -1,
			IsApproval: false,
		},
		{
			AccountID:  destinationId,
			Amount:     int64(amount.Uint64()),
			IsApproval: false,
		},
	}

	var tokenTransfers []*common.TokenTransferList
	contract, ok := args.GetContract()
	if ok {
		tokenId, err := commontypes.NewTokenId(string(contract))
		if err != nil {
			return nil, fmt.Errorf("failed to convert contract to token id: %w", err)
		}
		tokenTransfers = []*common.TokenTransferList{
			{
				Token:            tokenId,
				Transfers:        accountAmounts,
				NftTransfers:     []*common.NftTransfer{},
				ExpectedDecimals: &wrapperspb.UInt32Value{},
			},
		}
		accountAmounts = nil
	}

	cryptoTransferBody := &services.TransactionBody_CryptoTransfer{
		CryptoTransfer: &services.CryptoTransferTransactionBody{
			Transfers: &common.TransferList{
				AccountAmounts: accountAmounts,
			},
			TokenTransfers: tokenTransfers,
		},
	}

	body := &services.TransactionBody{
		TransactionID:  txi.TransactionId,
		NodeAccountID:  txi.NodeId,
		TransactionFee: txi.MaxFee,
		TransactionValidDuration: &services.Duration{
			Seconds: txi.ValidTime,
		},
		Memo: txi.Memo,
		Data: cryptoTransferBody,
	}

	return &Tx{
		TxBody:   body,
		SignedTx: &services.SignedTransaction{},
	}, nil
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	txBody, err := proto.Marshal(tx.SignedTx)
	if err != nil {
		return xc.TxHash("")
	}

	hash := sha512.Sum384(txBody)
	hexHash := fmt.Sprintf("0x%s", hex.EncodeToString(hash[:]))
	return xc.TxHash(hexHash)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	txBody, err := proto.Marshal(tx.TxBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tx body: %w", err)
	}
	tx.SignedTx.BodyBytes = txBody
	hash := sha3.NewLegacyKeccak256()
	hash.Write(txBody)
	h := hash.Sum(nil)

	return []*xc.SignatureRequest{
		{
			Payload: h,
		},
	}, nil

}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != 1 {
		return fmt.Errorf("invalid signature count, got: %d, expected: 1", len(signatures))
	}
	if tx.SignedTx.SigMap != nil {
		return errors.New("already signed")
	}

	signature := signatures[0]
	tx.SignedTx.SigMap = &common.SignatureMap{
		SigPair: []*common.SignaturePair{
			{
				Signature: &common.SignaturePair_ECDSASecp256K1{
					ECDSASecp256K1: signature.Signature[0:64],
				},
				// We can skip public key prefix because we are probiding only one signature
				// NOTE: ECDSA pubkey is required in compressed format
				// PubKeyPrefix: signature.PublicKey,
			},
		},
	}
	return nil
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	bytes, err := proto.Marshal(tx.SignedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed transaction: %w", err)
	}

	return bytes, nil
}
