package tx_input

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	// injectivecryptocodec "github.com/InjectiveLabs/sdk-go/chain/crypto/codec"
)

type CosmoAssetType string

// Cosmos assets can be managed by completely different modules (e.g. cosmwasm cw20, x/bank, etc)
var CW20 CosmoAssetType = "cw20"
var BANK CosmoAssetType = "bank"

// TxInput for Cosmos
type TxInput struct {
	xc.TxInputEnvelope
	AccountNumber       uint64  `json:"account_number,omitempty"`
	Sequence            uint64  `json:"sequence,omitempty"`
	GasLimit            uint64  `json:"gas_limit,omitempty"`
	GasPrice            float64 `json:"gas_price,omitempty"`
	LegacyMemo          string  `json:"memo,omitempty"`
	LegacyFromPublicKey []byte  `json:"from_pubkey,omitempty"`
	TimeoutHeight       uint64  `json:"timeout_height"`

	AssetType CosmoAssetType `json:"asset_type,omitempty"`
	ChainId   string         `json:"chain_id,omitempty"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}
var _ xc.TxInputWithMemo = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverCosmos
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	input.GasPrice, _ = multiplier.Mul(decimal.NewFromFloat(input.GasPrice)).Float64()
	return nil
}

func (input *TxInput) GetMaxFee() (xc.AmountBlockchain, xc.ContractAddress) {
	gasPrice := decimal.NewFromFloat(input.GasPrice)
	// Use big int to avoid casting uint64 to int64
	gasLimitInt := big.NewInt(0)
	gasLimitInt.SetUint64(input.GasLimit)
	gasLimit := decimal.NewFromBigInt(gasLimitInt, 0)

	// gasPrice * gasLimit
	totalSpend := gasPrice.Mul(gasLimit)
	totalSpendInt := xc.AmountBlockchain(*totalSpend.BigInt())

	// TODO: consider alt assets fees in cosmos
	return totalSpendInt, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if cosmosOther, ok := other.(*TxInput); ok {
		// cosmos address could own multiple accounts too which each have independent sequence.
		return cosmosOther.AccountNumber != input.AccountNumber ||
			cosmosOther.Sequence != input.Sequence
	}
	return
}
func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	// all same sequence means no double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// sequence all same - we're safe
	return true
}
func (txInput *TxInput) SetPublicKey(publicKeyBytes []byte) error {
	txInput.LegacyFromPublicKey = publicKeyBytes
	return nil
}

func (txInput *TxInput) SetMemo(memo string) {
	txInput.LegacyMemo = memo
}

func (txInput *TxInput) SetPublicKeyFromStr(publicKeyStr string) error {
	var publicKeyBytes []byte
	var err error
	if strings.HasPrefix(publicKeyStr, "0x") {
		publicKeyBytes, err = hex.DecodeString(publicKeyStr)
	} else {
		publicKeyBytes, err = base64.StdEncoding.DecodeString(publicKeyStr)
	}
	if err != nil {
		return fmt.Errorf("invalid public key %v: %v", publicKeyStr, err)
	}
	return txInput.SetPublicKey(publicKeyBytes)
}

// NewTxInput returns a new Cosmos TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverCosmos),
	}
}
