package api

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type AccountStatus string

var Active AccountStatus = "active"
var Uninit AccountStatus = "uninit"
var Nonexist AccountStatus = "nonexist"

type GetAccountResponse struct {
	Balance             string              `json:"balance"`
	Code                string              `json:"code"`
	Data                string              `json:"data"`
	LastTransactionLt   xc.AmountBlockchain `json:"last_transaction_lt"`
	LastTransactionHash string              `json:"last_transaction_hash"`
	FrozenHash          string              `json:"frozen_hash"`
	Status              AccountStatus       `json:"status"`
}

type Detail struct {
	Loc  []interface{} `json:"loc"`
	Msg  string        `json:"msg"`
	Type string        `json:"type"`
}

type ErrorResponse struct {
	// API doc specifies "details" as being in the response but in practice
	// it seems just a single error string is returned
	ErrorMessage string   `json:"error"`
	Detail       []Detail `json:"detail"`
	// should be set by http client
	StatusCode int `json:"-"`
}

func (e *ErrorResponse) Error() string {
	if e.ErrorMessage != "" {
		return fmt.Sprintf("%s", e.ErrorMessage)
	}
	if len(e.Detail) > 0 {
		return fmt.Sprintf("%s: %s", e.Detail[0].Type, e.Detail[0].Msg)
	}
	return fmt.Sprintf("unknown ton error (%d)", e.StatusCode)
}

type JettonWallet struct {
	Address           string              `json:"address"`
	Balance           string              `json:"balance"`
	Owner             string              `json:"owner"`
	Jetton            string              `json:"jetton"`
	LastTransactionLt xc.AmountBlockchain `json:"last_transaction_lt"`
	CodeHash          string              `json:"code_hash"`
	DataHash          string              `json:"data_hash"`
}

type JettonWalletsResponse struct {
	JettonWallets []JettonWallet `json:"jetton_wallets"`
}

type BlockRef struct {
	Workchain int    `json:"workchain"`
	Shard     string `json:"shard"`
	Seqno     int64  `json:"seqno"`
}

type Block struct {
	Workchain              int                 `json:"workchain"`
	Shard                  string              `json:"shard"`
	Seqno                  int64               `json:"seqno"`
	RootHash               string              `json:"root_hash"`
	FileHash               string              `json:"file_hash"`
	GlobalID               int                 `json:"global_id"`
	Version                int                 `json:"version"`
	AfterMerge             bool                `json:"after_merge"`
	BeforeSplit            bool                `json:"before_split"`
	AfterSplit             bool                `json:"after_split"`
	WantMerge              bool                `json:"want_merge"`
	WantSplit              bool                `json:"want_split"`
	KeyBlock               bool                `json:"key_block"`
	VertSeqnoIncr          bool                `json:"vert_seqno_incr"`
	Flags                  int                 `json:"flags"`
	GenUtime               string              `json:"gen_utime"`
	StartLT                xc.AmountBlockchain `json:"start_lt"`
	EndLT                  xc.AmountBlockchain `json:"end_lt"`
	ValidatorListHashShort int                 `json:"validator_list_hash_short"`
	GenCatchainSeqno       int                 `json:"gen_catchain_seqno"`
	MinRefMcSeqno          int                 `json:"min_ref_mc_seqno"`
	PrevKeyBlockSeqno      int                 `json:"prev_key_block_seqno"`
	VertSeqno              int                 `json:"vert_seqno"`
	MasterRefSeqno         int                 `json:"master_ref_seqno"`
	RandSeed               string              `json:"rand_seed"`
	CreatedBy              string              `json:"created_by"`
	TxCount                int                 `json:"tx_count"`
	MasterchainBlockRef    BlockRef            `json:"masterchain_block_ref"`
	PrevBlocks             []BlockRef          `json:"prev_blocks"`
}

type MasterChainInfo struct {
	Last  Block `json:"last"`
	First Block `json:"first"`
}

type StackItem struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type GetMethodResponse struct {
	GasUsed  int         `json:"gas_used"`
	ExitCode int         `json:"exit_code"`
	Stack    []StackItem `json:"stack"`
}

type GetMethod string

var GetPublicKeyMethod GetMethod = "get_public_key"
var GetSequenceMethod GetMethod = "seqno"
var GetWalletAddressMethod GetMethod = "get_wallet_address"
var GetJettonDataMethod GetMethod = "get_jetton_data"

type GetMethodRequest struct {
	Address string      `json:"address"`
	Method  GetMethod   `json:"method"`
	Stack   []StackItem `json:"stack"`
}

type SubmitMessageRequest struct {
	Boc string `json:"boc"`
}
type SubmitMessageResponse struct {
	MessageHash string `json:"message_hash"`
}

type Fees struct {
	InFwdFee   int64 `json:"in_fwd_fee"`
	StorageFee int64 `json:"storage_fee"`
	GasFee     int64 `json:"gas_fee"`
	FwdFee     int64 `json:"fwd_fee"`
}

func (f *Fees) Sum() int64 {
	return f.InFwdFee + f.StorageFee + f.GasFee + f.FwdFee
}

type FeeEstimateResponse struct {
	SourceFees      Fees   `json:"source_fees"`
	DestinationFees []Fees `json:"destination_fees"`
}

func (f *FeeEstimateResponse) Sum() int64 {
	sum := int64(0)
	sum += f.SourceFees.Sum()
	for _, dst := range f.DestinationFees {
		sum += dst.Sum()
	}
	return sum
}

type FeeEstimateRequest struct {
	Address string `json:"address"`
	Body    string `json:"body"`
}

type JettonMastersResponse struct {
	JettonMasters []JettonMaster         `json:"jetton_masters"`
	AddressBook   map[string]AddressInfo `json:"address_book"`
}

type JettonMaster struct {
	Address              string        `json:"address"`
	TotalSupply          string        `json:"total_supply"`
	Mintable             bool          `json:"mintable"`
	AdminAddress         *string       `json:"admin_address"`
	JettonContent        JettonContent `json:"jetton_content"`
	JettonWalletCodeHash string        `json:"jetton_wallet_code_hash"`
	CodeHash             string        `json:"code_hash"`
	DataHash             string        `json:"data_hash"`
	LastTransactionLT    string        `json:"last_transaction_lt"`
}

type JettonContent struct {
	Decimals xc.AmountBlockchain `json:"decimals"`
	URI      string              `json:"uri"`
}

type AddressInfo struct {
	UserFriendly string `json:"user_friendly"`
}
