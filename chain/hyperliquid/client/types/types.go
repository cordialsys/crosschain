package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	ActionSpotSend = "spotSend"
	ActionUsdSend  = "usdSend"
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

type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

type CrossMarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

type ClearinghouseState struct {
	MarginSummary              MarginSummary      `json:"marginSummary"`
	CrossMarginSummary         CrossMarginSummary `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string             `json:"crossMaintenanceMarginUsed"`
	Withdrawable               string             `json:"withdrawable"`
	AssetPositions             []interface{}      `json:"assetPositions"`
	Time                       int64              `json:"time"`
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

type Action interface {
	GetTime() uint64
	GetTypedData() (apitypes.TypedData, error)
}

type SpotSend struct {
	Type             string `json:"type"        msgpack:"type"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	Destination      string `json:"destination" msgpack:"destination"`
	Token            string `json:"token"       msgpack:"token"`
	Amount           string `json:"amount"      msgpack:"amount"`
	Time             uint64 `json:"time"        msgpack:"time"`
}

func (s SpotSend) GetTime() uint64 {
	return s.Time
}

func (s SpotSend) GetTypedData() (apitypes.TypedData, error) {
	amount := s.Amount

	chainId, err := strconv.ParseInt(s.SignatureChainId, 0, 64)
	if err != nil {
		return apitypes.TypedData{}, nil
	}
	hexChainId := math.HexOrDecimal256(*big.NewInt(chainId))
	typedData := apitypes.TypedData{
		Domain: apitypes.TypedDataDomain{
			ChainId:           &hexChainId,
			Name:              "HyperliquidSignTransaction",
			Version:           "1",
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Types: apitypes.Types{
			"HyperliquidTransaction:SpotSend": []apitypes.Type{
				{Name: "hyperliquidChain", Type: "string"},
				{Name: "destination", Type: "string"},
				{Name: "token", Type: "string"},
				{Name: "amount", Type: "string"},
				{Name: "time", Type: "uint64"},
			},
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
		},
		PrimaryType: "HyperliquidTransaction:SpotSend",
		Message: map[string]any{
			"hyperliquidChain": s.HyperliquidChain,
			"destination":      s.Destination,
			"token":            s.Token,
			"amount":           amount,
			"time":             big.NewInt(int64(s.Time)),
		},
	}

	return typedData, nil
}

type UsdcSend struct {
	Type             string `json:"type"        msgpack:"type"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	Destination      string `json:"destination" msgpack:"destination"`
	Amount           string `json:"amount"      msgpack:"amount"`
	Time             uint64 `json:"time"        msgpack:"time"`
}

func (s UsdcSend) GetTime() uint64 {
	return s.Time
}

func (s UsdcSend) GetTypedData() (apitypes.TypedData, error) {
	amount := s.Amount

	chainId, err := strconv.ParseInt(s.SignatureChainId, 0, 64)
	if err != nil {
		return apitypes.TypedData{}, nil
	}
	hexChainId := math.HexOrDecimal256(*big.NewInt(chainId))
	typedData := apitypes.TypedData{
		Domain: apitypes.TypedDataDomain{
			ChainId:           &hexChainId,
			Name:              "HyperliquidSignTransaction",
			Version:           "1",
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Types: apitypes.Types{
			"HyperliquidTransaction:UsdSend": []apitypes.Type{
				{Name: "hyperliquidChain", Type: "string"},
				{Name: "destination", Type: "string"},
				{Name: "amount", Type: "string"},
				{Name: "time", Type: "uint64"},
			},
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
		},
		PrimaryType: "HyperliquidTransaction:UsdSend",
		Message: map[string]any{
			"hyperliquidChain": s.HyperliquidChain,
			"destination":      s.Destination,
			"amount":           amount,
			"time":             big.NewInt(int64(s.Time)),
		},
	}

	return typedData, nil
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

func (t Transaction) IsSpotSend() bool {
	actionType, _ := GetValue[string](t.Action, "type")
	return actionType == ActionSpotSend
}

func (t Transaction) GetContract() xc.ContractAddress {
	token, _ := GetValue[string](t.Action, "token")
	return xc.ContractAddress(token)
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
		Type:             actionType,
		SignatureChainId: sigChainId,
		HyperliquidChain: hypeChain,
		Destination:      destination,
		Token:            token,
		Amount:           amount,
		Time:             uint64(timestamp),
	}, true, nil
}

func GetActionHash(action Action) (string, error) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetSortMapKeys(true)
	enc.UseCompactInts(true)
	err := enc.Encode(action)
	if err != nil {
		return "", fmt.Errorf("failed to encode action: %w", err)
	}

	data := buf.Bytes()

	timestamp := action.GetTime()
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, uint64(timestamp))
	data = append(data, nonceBytes...)

	// Append vault address, in our case "0x0"
	data = append(data, 0x00)
	hash := crypto.Keccak256Hash(data)
	hexHash := fmt.Sprintf("0x%x", hash)
	return hexHash, nil
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

type APIResponse struct {
	Status   string `json:"status"`
	Response any    `json:"response"`
}

func (r APIResponse) IsOk() bool {
	return r.Status == "ok"
}

type APIError struct {
	// "err"
	Status   string `json:"status"`
	Response string `json:"response"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Response)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

type UserDetails struct {
	Txs []Transaction `json:"txs,omitempty"`
}
