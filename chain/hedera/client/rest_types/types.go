package resttypes

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
)

const KEY_TIMESTAMP = "timestamp"

type TokenBalance struct {
	TokenId string `json:"token_id"`
	Balance uint64 `json:"balance"`
}

type Balance struct {
	// native hbar balance
	Balance uint64         `json:"balance"`
	Tokens  []TokenBalance `json:"tokens"`
}

type AccountInfo struct {
	Account            string    `json:"account"`
	Balance            Balance   `json:"balance"`
	ConsensusTimestamp Timestamp `json:"consensus_timestamp"`
}

type TokenInfo struct {
	TokenId  string `json:"token_id"`
	Decimals string `json:"decimals"`
}

type Timestamp string

func (t Timestamp) ToUnix() (time.Time, error) {
	parts := strings.SplitN(string(t), ".", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp: %s", t)
	}

	seconds, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse seconds: %w", err)
	}

	nanoSeconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse nanoseconds: %w", err)
	}

	return time.Unix(int64(seconds), int64(nanoSeconds)), nil
}

type BlockTimestamps struct {
	From Timestamp `json:"from"`
	To   Timestamp `json:"to"`
}

func (t BlockTimestamps) ToParam() (string, string) {
	return KEY_TIMESTAMP, fmt.Sprintf("lt:%s", t.To)
}

func (t BlockTimestamps) FromParam() (string, string) {
	return KEY_TIMESTAMP, fmt.Sprintf("gt:%s", t.From)
}

type Block struct {
	Hash      string          `json:"hash"`
	Timestamp BlockTimestamps `json:"timestamp"`
	Count     uint64          `json:"count"` // Transaction count
	Number    uint64          `json:"number"`
}

func (b Block) GetBlockTime() (time.Time, error) {
	return b.Timestamp.To.ToUnix()
}

type BlocksInfo struct {
	Blocks []Block `json:"blocks"`
}

type Transfer struct {
	Account    string `json:"account"`
	Amount     int64  `json:"amount"`
	IsApproval bool   `json:"is_approval"`
	// HBAR transfer if empty
	TokenId string `json:"token_id,omitempty"`
}

// Transaction format: {payerAccountId}-{timestampaSeconds}-{nanos}
// Example: 0.0.610168-1762779566-874119842
type TransactionId string

func (t TransactionId) GetPayerAccount() (string, error) {
	parts := strings.Split(string(t), "-")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid transaction id format: %s", t)
	}

	return parts[0], nil
}

type Transaction struct {
	ConsensusTimestamp string        `json:"consensus_timestamp"`
	ChargedTxFee       uint64        `json:"charged_tx_fee"`
	HashBase64         string        `json:"transaction_hash"`
	MemoBase64         string        `json:"memo_base64"`
	Node               string        `json:"node"`
	Nonce              int32         `json:"nonce"`
	Result             string        `json:"result"`
	TokenTransfers     []Transfer    `json:"token_transfers"`
	TransactionId      TransactionId `json:"transaction_id"`
	Transfers          []Transfer    `json:"transfers"`
}

func (t Transaction) GetSourceAddress() (string, error) {
	return t.TransactionId.GetPayerAccount()
}

func (t Transaction) BlockTimeParam() (string, string) {
	return KEY_TIMESTAMP, "gt:" + t.ConsensusTimestamp
}

func (t Transaction) GetMemo() (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(t.MemoBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode memo: %w", err)
	}
	return string(decoded), nil
}

func (t Transaction) GetHash() (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(t.HashBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode tx hash: %w", err)
	}
	hexHash := hex.EncodeToString(decoded)
	return "0x" + hexHash, nil
}

type Links struct {
	Next string `json:"next"`
}

type TransactionsInfo struct {
	Transactions []Transaction `json:"transactions"`
	Links        Links         `json:"links"`
}

type Rate struct {
	CentEquivalent uint64 `json:"cent_equivalent"`
	HbarEquivalent uint64 `json:"hbar_equivalent"`
	ExpirationTime uint64 `json:"expiration_time"`
}

func (r Rate) GetHbarEquivalent(usd xc.AmountHumanReadable) xc.AmountHumanReadable {
	xcCentsEq := xc.NewAmountBlockchainFromUint64(r.CentEquivalent)
	hrCentsEq := xc.NewAmountHumanReadableFromBlockchain(xcCentsEq)
	xcHbarEq := xc.NewAmountBlockchainFromUint64(r.HbarEquivalent)
	// equivalent is in HBARs, not tinybars
	hrHbarEq := xcHbarEq.ToHuman(0)

	hrCentToHbar := hrHbarEq.Div(hrCentsEq)
	// make sure to convert to USD before getting a ratio
	hrUsdToHbar := hrCentToHbar.Mul(xc.NewAmountHumanReadableFromFloat(100.0))
	return hrUsdToHbar.Mul(usd)
}

type ExchangeRate struct {
	CurrentRate Rate      `json:"current_rate"`
	NextRate    Rate      `json:"next_rate"`
	Timestamp   Timestamp `json:"timestamp"`
}

func (e ExchangeRate) GetMaxEquivalent(usd xc.AmountHumanReadable) xc.AmountHumanReadable {
	currEq := e.CurrentRate.GetHbarEquivalent(usd)
	nextEq := e.NextRate.GetHbarEquivalent(usd)

	if currEq.Cmp(nextEq) == 1 {
		return currEq
	} else {
		return nextEq
	}
}

// Main error response structure
type ErrorResponse struct {
	Status *ErrorStatus `json:"_status,omitempty"`
	Err    string       `json:"error,omitempty"` // Simple format
}

var _ error = &ErrorResponse{}

type ErrorStatus struct {
	Messages []ErrorMessage `json:"messages"`
}

type ErrorMessage struct {
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Data    string `json:"data,omitempty"`
}

func (r ErrorResponse) Error() string {
	str := ""
	if r.Status != nil {
		for _, m := range r.Status.Messages {
			str += m.Message
			if len(m.Detail) > 0 {
				str += ": " + m.Detail
			}

			if len(m.Data) > 0 {
				str += ", data: " + m.Detail
			}
		}
	} else {
		str = r.Err
	}
	return str
}
