package tx

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/vmihailenco/msgpack/v5"
)

const ActionSpotSend = "spotSend"

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
	Amount             xc.AmountBlockchain
	Decimals           int32
	Destination        xc.Address
	Token              xc.ContractAddress
	Nonce              int64
	PhantomAgentSource tx_input.PhantomAgentSource
	Signature          SignatureResult
}

var _ xc.Tx = &Tx{}

func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) Tx {
	return Tx{
		Amount:             args.GetAmount(),
		Decimals:           input.Decimals,
		Destination:        args.GetTo(),
		Token:              input.Token,
		Nonce:              input.TransactionTime,
		PhantomAgentSource: input.Source,
	}
}

// SpotTransferAction represents spot transfer
type SpotTransferAction struct {
	Type             string `json:"type"        msgpack:"type"`
	SignatureChainId string `json:"signatureChainId"` // msgpack:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	Destination      string `json:"destination" msgpack:"destination"`
	Token            string `json:"token"       msgpack:"token"`
	Amount           string `json:"amount"      msgpack:"amount"`
	Time             int64  `json:"time"        msgpack:"time"`
}

func (tx Tx) GetAction() SpotTransferAction {
	amount := tx.Amount.ToHuman(tx.Decimals)
	return SpotTransferAction{
		Type:             ActionSpotSend,
		SignatureChainId: "0xa4b1",
		HyperliquidChain: "Mainnet",
		Destination:      string(tx.Destination),
		Token:            string(tx.Token),
		Amount:           amount.String(),
		Time:             int64(tx.Nonce),
	}
}

func (tx Tx) GetActionHash() ([]byte, error) {
	action := tx.GetAction()

	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetSortMapKeys(true)
	enc.UseCompactInts(true)
	err := enc.Encode(action)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action: %w", err)
	}

	data := buf.Bytes()

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, uint64(tx.Nonce))
	data = append(data, nonceBytes...)

	// Append vault address, in our case "0x0"
	data = append(data, 0x00)
	hash := crypto.Keccak256Hash(data)
	return hash[:], nil
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	return xc.TxHash("not implemented")
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	action := tx.GetAction()

	chainId := math.HexOrDecimal256(*big.NewInt(42161))
	typedData := apitypes.TypedData{
		Domain: apitypes.TypedDataDomain{
			ChainId:           &chainId,
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
			"hyperliquidChain": "Mainnet",
			"destination":      string(tx.Destination),
			"token":            string(tx.Token),
			"amount":           action.Amount,
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
