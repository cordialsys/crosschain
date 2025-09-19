package tx

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hyperliquid/client/types"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

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
	Nonce            uint64
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
		Nonce:            uint64(input.TransactionTime.UnixMilli()),
		HyperliquidChain: input.HyperliquidChain,
	}
}

func (tx Tx) GetAction() types.Action {
	amount := tx.Amount.ToHuman(tx.Decimals)

	if len(tx.Token) == 0 {
		return types.UsdSend{
			Type:             types.ActionUsdSend,
			SignatureChainId: SignatureChainId,
			HyperliquidChain: tx.HyperliquidChain,
			Destination:      string(tx.Destination),
			Amount:           amount.String(),
			Time:             tx.Nonce,
		}
	} else {
		return types.SpotSend{
			Type:             types.ActionSpotSend,
			SignatureChainId: SignatureChainId,
			HyperliquidChain: tx.HyperliquidChain,
			Destination:      string(tx.Destination),
			Token:            string(tx.Token),
			Amount:           amount.String(),
			Time:             tx.Nonce,
		}
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
	typedData, err := tx.GetAction().GetTypedData()

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
