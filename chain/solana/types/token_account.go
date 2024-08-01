package types

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go/rpc"
)

// This is the value.account.data
// object returned from getTokenAccountsByOwner
type TokenAccountInfo struct {
	Parsed  TokenAccountInfoParsed `json:"parsed"`
	Program string                 `json:"program"`
	Space   uint64                 `json:"space"`
}
type TokenAccountInfoParsed struct {
	Info TokenAccountInfoParsedInfo `json:"info"`
	Type string                     `json:"type"`
}
type TokenAccountInfoParsedInfo struct {
	IsNative    bool                                  `json:"isNative"`
	Mint        string                                `json:"mint"`
	Owner       string                                `json:"owner"`
	State       string                                `json:"state"`
	TokenAmount TokenAccountInfoParsedInfoTokenAmount `json:"tokenAmount"`
}
type TokenAccountInfoParsedInfoTokenAmount struct {
	Amount       string  `json:"amount"`
	Decimals     uint64  `json:"decimals"`
	UinAmount    float32 `json:"uiAmount"`
	UinAmountStr string  `json:"uiAmountString"`
}

// Parse from json data returned by solana client.
// Note the client request must use `Encoding:   "jsonParsed"` option.
func ParseRpcData[T TokenAccountInfo](data *rpc.DataBytesOrJSON) (T, error) {
	var info T
	rawJson, err := data.MarshalJSON()
	if err != nil {
		return info, errors.Join(fmt.Errorf("could not parse %T", info), err)
	}
	err = json.Unmarshal(rawJson, &info)
	if err != nil {
		return info, errors.Join(fmt.Errorf("could not marshal raw %T", info), err)
	}
	return info, nil
}
