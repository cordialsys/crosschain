package types

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

const (
	ActionSpotSend = "spotSend"
)

type SpotClearinghouseState struct {
	Balances []SpotBalance `json:"balances"`
}

type SpotBalance struct {
	Coin     string `json:"coin"`
	Token    int    `json:"token"`
	Total    string `json:"total"`
	Hold     string `json:"hold"`
	EntryNtl string `json:"entryNtl"`
}

type SpotMetaResponse struct {
	Universe []TradingPair `json:"universe"`
	Tokens   []Token       `json:"tokens"`
}

func (s SpotMetaResponse) GetTokenMetaByName(name string) (Token, bool) {
	for _, token := range s.Tokens {
		if token.Name == name {
			return token, true
		}
	}

	return Token{}, false
}

func (s SpotMetaResponse) GetTokenMetaByTokenId(contract xc.ContractAddress) (Token, bool) {
	for _, token := range s.Tokens {
		if token.TokenId == string(contract) {
			return token, true
		}
	}

	return Token{}, false
}

type TradingPair struct {
	Tokens      []int  `json:"tokens"`
	Name        string `json:"name"`  // "@107" format for trading pair ID
	Index       int    `json:"index"` // Trading pair index
	IsCanonical bool   `json:"isCanonical"`
}

type Token struct {
	Name                    string       `json:"name"`                    // "HYPE", "USDC"
	SzDecimals              int          `json:"szDecimals"`              // Trading precision (0-2)
	WeiDecimals             int          `json:"weiDecimals"`             // Token precision (0-8)
	Index                   int          `json:"index"`                   // Numeric token identifier
	TokenId                 string       `json:"tokenId"`                 // HyperCore token address
	IsCanonical             bool         `json:"isCanonical"`             // Official/canonical token
	EvmContract             *EvmContract `json:"evmContract"`             // HyperEVM bridge info
	FullName                string       `json:"fullName"`                // Full token name
	DeployerTradingFeeShare string       `json:"deployerTradingFeeShare"` // Fee share percentage
}

type EvmContract struct {
	Address             string `json:"address"`                // HyperEVM ERC20 address
	EvmExtraWeiDecimals int    `json:"evm_extra_wei_decimals"` // Additional decimals on EVM
}

type SpotSend struct {
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
	Destination      string `json:"destination"`
	Token            string `json:"token"`
	Amount           string `json:"amount"`
	Time             int64  `json:"time"`
}

type Transaction struct {
	Time   int64          `json:"time"`
	User   string         `json:"user"`
	Action map[string]any `json:"action,omitempty"`
	Block  uint64         `json:"block"`
	Hash   string         `json:"hash"`
	Error  string         `json:"error"`
}

func GetValue[T any](m map[string]any, key string) (T, bool) {
	var r T

	v, ok := m[key]
	if !ok {
		return r, false
	}

	if c, ok := v.(T); ok {
		return c, true
	}

	return r, false
}

func (t Transaction) GetSpotSend() (SpotSend, bool, error) {
	actionType, ok := GetValue[string](t.Action, "type")
	if !ok {
		return SpotSend{}, false, errors.New("invalid action format")
	}

	if actionType != ActionSpotSend {
		return SpotSend{}, false, nil
	}

	sigChainId, ok := GetValue[string](t.Action, "signatureChainId")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'signatureChainId'")
	}
	hypeChain, ok := GetValue[string](t.Action, "hyperliquidChain")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'hyperliquidChain'")
	}
	destination, ok := GetValue[string](t.Action, "destination")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'destination'")
	}

	token, ok := GetValue[string](t.Action, "token")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'token'")
	}
	amount, ok := GetValue[string](t.Action, "amount")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'amount'")
	}

	// json numbers are always float
	timestamp, ok := GetValue[float64](t.Action, "time")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'time'")
	}

	return SpotSend{
		SignatureChainId: sigChainId,
		HyperliquidChain: hypeChain,
		Destination:      destination,
		Token:            token,
		Amount:           amount,
		Time:             int64(timestamp),
	}, true, nil
}

type BlockDetails struct {
	Height    uint64        `json:"height"`
	BlockTime int64         `json:"blockTime"`
	Hash      string        `json:"hash"`
	Proposer  string        `json:"proposer"`
	NumTxs    uint64        `json:"numTxs"`
	Txs       []Transaction `json:"txs,omitempty"`
}

type UserNonFundingLedgerUpdate struct {
	Time  int64          `json:"time,omitempty"`
	Hash  string         `json:"hash,omitempty"`
	Delta map[string]any `json:"delta,omitempty"`
}

func (u UserNonFundingLedgerUpdate) GetFee() string {
	fee, ok := GetValue[string](u.Delta, "fee")
	if ok {
		return fee
	}

	nativeFee, ok := GetValue[string](u.Delta, "nativeTokenFee")
	if ok {
		return nativeFee
	}

	return "0.0"
}

func (u UserNonFundingLedgerUpdate) GetFeeToken() string {
	feeToken, _ := GetValue[string](u.Delta, "feeToken")
	return feeToken
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Data    any    `json:"data,omitempty"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}
