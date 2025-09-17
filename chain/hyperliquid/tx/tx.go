package tx

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client/types"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const ActionSpotSend = "spotSend"
const SignatureChainId = "0xa4b1"

// SignatureResult represents the structured signature result
type SignatureResult struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

func (s SignatureResult) IsEmpty() bool {
	return s.R == "" && s.S == "" && s.V == 0
}

// Tx for Template
type Tx struct {
	Amount           xc.AmountBlockchain
	Decimals         int32
	Destination      xc.Address
	Token            xc.ContractAddress
	Nonce            int64
	HyperliquidChain string
	Signature        SignatureResult
}

var _ xc.Tx = &Tx{}

func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) Tx {
	return Tx{
		Amount:           args.GetAmount(),
		Decimals:         input.Decimals,
		Destination:      args.GetTo(),
		Token:            input.Token,
		Nonce:            input.TransactionTime,
		HyperliquidChain: input.HyperliquidChain,
	}
}

// SpotTransferAction represents spot transfer
type SpotTransferAction struct {
	Type             string `json:"type"        msgpack:"type"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	Destination      string `json:"destination" msgpack:"destination"`
	Token            string `json:"token"       msgpack:"token"`
	Amount           string `json:"amount"      msgpack:"amount"`
	Time             int64  `json:"time"        msgpack:"time"`
}

func (tx Tx) GetAction() map[string]any {
	amount := tx.Amount.ToHuman(tx.Decimals)

	return map[string]any{
		"type":             ActionSpotSend,
		"signatureChainId": SignatureChainId,
		"hyperliquidChain": tx.HyperliquidChain,
		"destination":      string(tx.Destination),
		"token":            string(tx.Token),
		"amount":           amount.String(),
		"time":             int64(tx.Nonce),
	}
}

func (tx Tx) GetActionHash() (string, error) {
	action := tx.GetAction()
	return types.GetActionHash(action)
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	hash, err := tx.GetActionHash()
	if err != nil {
		return xc.TxHash("")
	}
	return xc.TxHash(hash)
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	// amount := tx.Amount.ToHuman(tx.Decimals)

	chainId, err := strconv.ParseInt(SignatureChainId, 0, 64)
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
			"hyperliquidChain": tx.HyperliquidChain,
			"destination":      string(tx.Destination),
			"token":            string(tx.Token),
			"amount":           "0.1",
			"time":             big.NewInt(tx.Nonce),
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("failed to hash domain: %w", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to hash typed data: %w", err)
	}

	rawData := []byte{0x19, 0x01}
	rawData = append(rawData, domainSeparator...)
	rawData = append(rawData, typedDataHash...)
	sighash := crypto.Keccak256Hash(rawData)

	return []*xc.SignatureRequest{
		{
			Payload: sighash.Bytes(),
		},
	}, nil
}

func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if !tx.Signature.IsEmpty() {
		return errors.New("already signed")
	}

	if len(signatures) != 1 {
		return fmt.Errorf("expected only 1 signature, got: %d", len(signatures))
	}

	signature := signatures[0].Signature
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:64])
	v := int(signature[64]) + 27
	tx.Signature = SignatureResult{
		R: hexutil.EncodeBig(r),
		S: hexutil.EncodeBig(s),
		V: v,
	}

	return nil
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	payload := map[string]any{
		"action":    tx.GetAction(),
		"nonce":     tx.Nonce,
		"signature": tx.Signature,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return jsonData, nil
}
