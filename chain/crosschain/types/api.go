package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

type Status status.Status

func (s *Status) Error() string {
	return fmt.Sprintf("%s: %s", codes.Code(s.Code).String(), s.Message)
}

type ApiResponse interface{}

type BalanceReq struct {
	Chain    xc.NativeAsset `json:"chain"`
	Contract string         `json:"contract,omitempty"`
	Address  string         `json:"address"`
}

type BalanceRes struct {
	*BalanceReq
	Balance xc.AmountBlockchain `json:"balance"`
	// legacy field for backwards compatibility, should use `.balance`
	XBalanceRaw xc.AmountBlockchain `json:"balance_raw"`
}

func (b *BalanceRes) GetBalance() xc.AmountBlockchain {
	// Use the legacy field if it's set
	if b.XBalanceRaw.Cmp(&b.Balance) > 0 {
		return b.XBalanceRaw
	}
	// otherwise use the new field
	return b.Balance
}

type TransferInputReq struct {
	Chain    xc.NativeAsset `json:"chain"`
	Contract string         `json:"contract,omitempty"`
	Decimals string         `json:"decimals,omitempty"`

	From    string `json:"from"`
	To      string `json:"to"`
	Balance string `json:"balance"`

	FeePayer *FeePayerInfo `json:"fee_payer,omitempty"`

	// Optional transfer parameters:
	// public key of the signing address, in hex
	PublicKey string `json:"public_key,omitempty"`
	Memo      string `json:"memo,omitempty"`
	Priority  string `json:"priority,omitempty"`

	PreviousTransactionAttempts []string `json:"previous_transaction_attempts,omitempty"`
}

type FeePayerInfo struct {
	// Address of the fee payer
	Address string `json:"address"`
	// Hex encoded public key
	PublicKey string `json:"public_key"`
}

type FeePayerGetter interface {
	GetFeePayer() (xc.Address, bool)
	GetFeePayerPublicKey() ([]byte, bool)
}

func NewFeePayerInfoOrNil(feePayerGetter FeePayerGetter) *FeePayerInfo {
	address, ok := feePayerGetter.GetFeePayer()
	if !ok {
		return nil
	}
	publicKey, ok := feePayerGetter.GetFeePayerPublicKey()
	if !ok {
		return nil
	}
	return &FeePayerInfo{Address: string(address), PublicKey: hex.EncodeToString(publicKey)}
}

type Sender struct {
	Address xc.Address `json:"address"`
	// hex-encoded
	PublicKey string `json:"public_key"`
}
type Receiver struct {
	Address  xc.Address          `json:"address"`
	Balance  xc.AmountBlockchain `json:"balance"`
	Memo     string              `json:"memo,omitempty"`
	Contract xc.ContractAddress  `json:"contract,omitempty"`
	Decimals int                 `json:"decimals,omitempty"`
}

type MultiTransferInputReq struct {
	Chain     xc.NativeAsset `json:"chain"`
	Senders   []*Sender      `json:"senders"`
	Receivers []*Receiver    `json:"receivers"`
	FeePayer  *FeePayerInfo  `json:"fee_payer,omitempty"`
	Priority  string         `json:"priority,omitempty"`
	Memo      string         `json:"memo,omitempty"`

	PreviousTransactionAttempts []string `json:"previous_transaction_attempts,omitempty"`
}

type StakingInputReq struct {
	From      string             `json:"from"`
	Balance   string             `json:"balance"`
	Validator string             `json:"validator,omitempty"`
	Account   string             `json:"account,omitempty"`
	Provider  xc.StakingProvider `json:"provider,omitempty"`
	FeePayer  *FeePayerInfo      `json:"fee_payer,omitempty"`
}

type LegacyTxInputRes struct {
	*TransferInputReq
	xc.TxInput `json:"raw_tx_input,omitempty"`
	NewTxInput json.RawMessage `json:"input,omitempty"`
}

type TransferInputRes struct {
	Input string `json:"input,omitempty"`
}

type TxInputRes struct {
	*TransferInputReq
	TxInput string `json:"input,omitempty"`
}

type TxInfoReq struct {
	Chain  xc.NativeAsset `json:"chain"`
	TxHash string         `json:"tx_hash"`
}

type TxLegacyInfoRes struct {
	*TxInfoReq
	xclient.LegacyTxInfo `json:"tx_info,omitempty"`
}

type TransactionInfoRes struct {
	xclient.TxInfo
}

type SubmitTxReq struct {
	Chain        xc.NativeAsset `json:"chain"`
	TxData       []byte         `json:"tx_data"`
	TxSignatures [][]byte       `json:"tx_signatures"`
}

type SubmitTxRes struct {
	*SubmitTxReq
}

var _ xc.Tx = &SubmitTxReq{}

func NewBinaryTx(serializedSignedTx []byte, TxSignatures [][]byte) xc.Tx {
	return &SubmitTxReq{
		TxData:       serializedSignedTx,
		TxSignatures: TxSignatures,
	}
}

func (tx *SubmitTxReq) Hash() xc.TxHash {
	panic("not implemented")
}
func (tx *SubmitTxReq) Sighashes() ([]*xc.SignatureRequest, error) {
	panic("not implemented")
}
func (tx *SubmitTxReq) SetSignatures(sigs ...*xc.SignatureResponse) error {
	for _, sig := range sigs {
		tx.TxSignatures = append(tx.TxSignatures, sig.Signature)
	}
	return nil
}
func (tx *SubmitTxReq) GetSignatures() []xc.TxSignature {
	sigs := []xc.TxSignature{}
	for _, sig := range tx.TxSignatures {
		sigs = append(sigs, sig)
	}
	return sigs
}
func (tx *SubmitTxReq) Serialize() ([]byte, error) {
	return tx.TxData, nil
}

type BlockResponse struct {
	Hash           string              `json:"hash"`
	Height         xc.AmountBlockchain `json:"height"`
	Time           *string             `json:"time,omitempty"`
	ChainId        string              `json:"chain_id"`
	TransactionIds []string            `json:"transaction_ids,omitempty"`
	SubBlocks      []*BlockResponse    `json:"sub_blocks,omitempty"`
}

func UnpackBlock(apiBlock *BlockResponse) (*xclient.BlockWithTransactions, error) {
	var t time.Time
	var err error
	if apiBlock.Time != nil {
		t, err = time.Parse(time.RFC3339, *apiBlock.Time)
		if err != nil {
			return nil, fmt.Errorf("invalid time: %v", err)
		}
	}
	block := &xclient.BlockWithTransactions{
		Block: xclient.Block{
			Chain:  xc.NativeAsset(apiBlock.ChainId),
			Height: apiBlock.Height,
			Hash:   apiBlock.Hash,
			Time:   t,
		},
		TransactionIds: apiBlock.TransactionIds,
	}
	return block, nil
}
func UnpackBlocksInner(apiBlocks []*BlockResponse) ([]*xclient.BlockWithTransactions, error) {
	if len(apiBlocks) == 0 {
		return []*xclient.BlockWithTransactions{}, nil
	}
	blocks := make([]*xclient.BlockWithTransactions, len(apiBlocks))
	var err error
	for i := range apiBlocks {
		blocks[i], err = UnpackBlock(apiBlocks[i])
		if err != nil {
			return blocks, nil
		}
	}

	return blocks, nil
}

func UnpackBlocks(apiBlocks []*BlockResponse) ([]*xclient.BlockWithTransactions, error) {
	blocks, err := UnpackBlocksInner(apiBlocks)
	if err != nil {
		return blocks, err
	}
	// Do we really need nesting for reporting blocks?
	// -> So far 1 level is fine.
	// only unnest 3 levels deep for now
	// for i := range apiBlocks {
	// 	blocks[i].SubBlocks, err = UnpackBlocksInner(apiBlocks[i].SubBlocks)
	// 	if err != nil {
	// 		return blocks, err
	// 	}
	// 	for j := range apiBlocks[i].SubBlocks {
	// 		blocks[i].SubBlocks[j].SubBlocks, err = UnpackBlocksInner(apiBlocks[i].SubBlocks[j].SubBlocks)
	// 		if err != nil {
	// 			return blocks, err
	// 		}
	// 	}
	// }

	return blocks, nil
}

func UnpackSubBlocks(apiBlocks []*BlockResponse) ([]*xclient.SubBlockWithTransactions, error) {
	if len(apiBlocks) == 0 {
		return []*xclient.SubBlockWithTransactions{}, nil
	}
	blocks := make([]*xclient.SubBlockWithTransactions, len(apiBlocks))
	for i := range apiBlocks {
		block, err := UnpackBlock(apiBlocks[i])
		if err != nil {
			return blocks, nil
		}
		blocks = append(blocks, &xclient.SubBlockWithTransactions{
			Block:          block.Block,
			TransactionIds: block.TransactionIds,
		})
	}

	return blocks, nil
}
