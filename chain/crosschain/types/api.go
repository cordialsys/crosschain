package types

import (
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/call"
	xclient "github.com/cordialsys/crosschain/client/tx_info"
	xclient_types "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/pkg/hex"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

// Optional query arguments for fetching tx info
const (
	QueryTxInfoContract    = "contract"
	QueryTxInfoSender      = "sender"
	QueryTxInfoSignTime    = "sign_time"
	QueryTxInfoBlockHeight = "block.height"
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

// Optional parameters, not used by most chains:
type TransferInputReqExtra struct {
	// "Identities" are non-public key accounts on chain.  Only used by EOS currently.
	FromIdentity     string `json:"from_identity,omitempty"`
	FeePayerIdentity string `json:"fee_payer_identity,omitempty"`
	// TransactionAttempts Currently is only used by EVM chains.
	TransactionAttempts []string `json:"transaction_attempts,omitempty"`
	// Memo and priority are not currently used.
	Memo     string `json:"memo,omitempty"`
	Priority string `json:"priority,omitempty"`
}

type TransferInputReq struct {
	Chain    xc.NativeAsset `json:"chain"`
	Contract string         `json:"contract,omitempty"`
	Decimals string         `json:"decimals,omitempty"`

	From    string `json:"from"`
	To      string `json:"to"`
	Balance string `json:"balance"`

	FeePayer  *FeePayerInfo `json:"fee_payer,omitempty"`
	PublicKey string        `json:"public_key,omitempty"`

	Extra TransferInputReqExtra `json:"extra,omitempty"`
}

type FeePayerInfo struct {
	// Address of the fee payer
	Address   string  `json:"address"`
	PublicKey hex.Hex `json:"public_key"`

	Identity string `json:"identity,omitempty"`
}

type FeePayerGetter interface {
	GetFeePayer() (xc.Address, bool)
	GetFeePayerPublicKey() ([]byte, bool)
	GetFeePayerIdentity() (string, bool)
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
	identity, _ := feePayerGetter.GetFeePayerIdentity()

	return &FeePayerInfo{Address: string(address), PublicKey: publicKey, Identity: identity}
}

type SenderExtra struct {
	// Optional parameters, not used by most chains:
	Identity string `json:"identity,omitempty"`
}
type Sender struct {
	Address   xc.Address `json:"address"`
	PublicKey hex.Hex    `json:"public_key"`

	Extra SenderExtra `json:"extra,omitempty"`
}
type Receiver struct {
	Address  xc.Address          `json:"address"`
	Balance  xc.AmountBlockchain `json:"balance"`
	Memo     string              `json:"memo,omitempty"`
	Contract xc.ContractAddress  `json:"contract,omitempty"`
	Decimals int                 `json:"decimals,omitempty"`
}

type MultiTransferInputReqExtra struct {
	Priority            string   `json:"priority,omitempty"`
	Memo                string   `json:"memo,omitempty"`
	TransactionAttempts []string `json:"transaction_attempts,omitempty"`
}

type MultiTransferInputReq struct {
	Chain     xc.NativeAsset `json:"chain"`
	Senders   []*Sender      `json:"senders"`
	Receivers []*Receiver    `json:"receivers"`
	FeePayer  *FeePayerInfo  `json:"fee_payer,omitempty"`

	Extra MultiTransferInputReqExtra `json:"extra,omitempty"`
}

type StakingInputReqExtra struct {
	// Optional parameters, not used by most chains:

	FromIdentity     string `json:"from_identity,omitempty"`
	FeePayerIdentity string `json:"fee_payer_identity,omitempty"`
}

type StakingInputReq struct {
	From          string               `json:"from"`
	FromPublicKey hex.Hex              `json:"from_public_key,omitempty"`
	Balance       string               `json:"balance,omitempty"`
	Validator     string               `json:"validator,omitempty"`
	Account       string               `json:"account,omitempty"`
	Provider      xc.StakingProvider   `json:"provider,omitempty"`
	FeePayer      *FeePayerInfo        `json:"fee_payer,omitempty"`
	Extra         StakingInputReqExtra `json:"extra,omitempty"`
	Memo          string               `json:"memo,omitempty"`
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

type SubmitTxRes struct {
	*xclient_types.SubmitTxReq
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

type CallInputReq struct {
	Method    call.Method     `json:"method"`
	Request   json.RawMessage `json:"request"`
	Addresses []xc.Address    `json:"addresses"`
}
