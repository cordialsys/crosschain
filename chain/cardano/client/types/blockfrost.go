package types

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

const (
	Cardano = "cardano"
	// Cardano uses lovelace as the smallest unit of ada
	// 1 lovelace = 0.000001 ada
	Lovelace                      = "lovelace"
	Ada                           = "ADA"
	CodeRequestNotValid           = "400"
	CodeDailyRequestLimitExceeded = "402"
	CodeNotAuthenticated          = "403"
	CodeResourceDoesNotExist      = "404"
	CodeUserAutoBannedForFlooding = "418"
	CodeMempoolFull               = "425"
	CodeRateLimitExceeded         = "429"
	CodeInternalServerError       = "500"
)

type Error struct {
	StatusCode int    `json:"status_code,omitempty"`
	Err        string `json:"error,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("StatusCode: %d, Error: %s, Message: %s", e.StatusCode, e.Err, e.Message)
}

type Amount struct {
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}

type GetAddressInfoResponse struct {
	Address      string   `json:"address"`
	Amounts      []Amount `json:"amount"`
	StakeAddress string   `json:"stake_address"`
}

type Block struct {
	Time          int64  `json:"time"`
	Height        uint64 `json:"height"`
	Hash          string `json:"hash"`
	Slot          uint64 `json:"slot"`
	Confirmations uint64 `json:"confirmations"`
}

type TransactionInfo struct {
	Hash        string `json:"hash"`
	Block       string `json:"block"`
	BlockHeight uint64 `json:"block_height"`
	BlockTime   uint64 `json:"block_time"`
	Fees        string `json:"fees"`
}

type ProtocolParameters struct {
	FeePerByte       uint64 `json:"min_fee_a"`
	FixedFee         uint64 `json:"min_fee_b"`
	MinUtxoValue     string `json:"min_utxo"`
	CoinsPerUtxoWord string `json:"coins_per_utxo_word"`
	KeyDeposit       string `json:"key_deposit"`
}

type Utxo struct {
	Address string   `json:"address"`
	Amounts []Amount `json:"amount"`
	TxHash  string   `json:"tx_hash"`
	Index   uint16   `json:"output_index"`
}

func (u *Utxo) GetAssetAmount(contract xc.ContractAddress) xc.AmountBlockchain {
	for _, amount := range u.Amounts {
		if amount.Unit == string(contract) {
			return xc.NewAmountBlockchainFromStr(amount.Quantity)
		}
	}

	return xc.NewAmountBlockchainFromUint64(0)
}

type TransactionUtxos struct {
	Inputs  []Utxo `json:"inputs"`
	Outputs []Utxo `json:"outputs"`
}

type GetAccountInfoResponse struct {
	WithdrawableAmount string `json:"withdrawable_amount"`
	PoolId             string `json:"pool_id"`
}
