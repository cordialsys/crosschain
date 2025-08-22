package types

import xc "github.com/cordialsys/crosschain"

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
