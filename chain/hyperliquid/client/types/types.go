package types

import (
	"errors"

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

func (s SpotMetaResponse) GetTokenMetaByContract(contract xc.ContractAddress) (Token, bool) {
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
	timestamp, ok := GetValue[int64](t.Action, "time")
	if !ok {
		return SpotSend{}, false, errors.New("failed to get spot send, missing: 'time'")
	}

	return SpotSend{
		SignatureChainId: sigChainId,
		HyperliquidChain: hypeChain,
		Destination:      destination,
		Token:            token,
		Amount:           amount,
		Time:             timestamp,
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
