package types

import (
	"encoding/json"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	"google.golang.org/genproto/googleapis/rpc/status"
)

type Status status.Status

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

type TxInputReq struct {
	Chain    xc.NativeAsset `json:"chain"`
	Contract string         `json:"contract,omitempty"`
	Decimals string         `json:"decimals,omitempty"`

	From    string `json:"from"`
	To      string `json:"to"`
	Balance string `json:"balance"`

	// Optional transfer parameters:
	// public key of the signing address, in hex
	PublicKey string `json:"public_key,omitempty"`
	Memo      string `json:"memo,omitempty"`
	Priority  string `json:"priority,omitempty"`
}

type StakingInputReq struct {
	From      string             `json:"from"`
	Balance   string             `json:"balance"`
	Validator string             `json:"validator,omitempty"`
	Account   string             `json:"account,omitempty"`
	Provider  xc.StakingProvider `json:"provider,omitempty"`
}

type LegacyTxInputRes struct {
	*TxInputReq
	xc.TxInput `json:"raw_tx_input,omitempty"`
	NewTxInput json.RawMessage `json:"input,omitempty"`
}

type TxInputRes struct {
	*TxInputReq
	TxInput string `json:"input,omitempty"`
}

type TxInfoReq struct {
	Chain  xc.NativeAsset `json:"chain"`
	TxHash string         `json:"tx_hash"`
}

type TxLegacyInfoRes struct {
	*TxInfoReq
	xc.LegacyTxInfo `json:"tx_info,omitempty"`
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
func (tx *SubmitTxReq) Sighashes() ([]xc.TxDataToSign, error) {
	panic("not implemented")
}
func (tx *SubmitTxReq) AddSignatures(sigs ...xc.TxSignature) error {
	for _, sig := range sigs {
		tx.TxSignatures = append(tx.TxSignatures, sig)
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
