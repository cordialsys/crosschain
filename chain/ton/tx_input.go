package ton

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/api"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
)

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	// MasterChainInfo api.MasterChainInfo `json:"master_chain_info"`
	AccountStatus   api.AccountStatus   `json:"account_status"`
	Sequence        uint64              `json:"sequence"`
	PublicKey       []byte              `json:"public_key,omitempty"`
	Memo            string              `json:"memo,omitempty"`
	Timestamp       int64               `json:"timestamp"`
	TokenWallet     xc.Address          `json:"token_wallet"`
	EstimatedMaxFee xc.AmountBlockchain `json:"estimated_max_fee"`
	TonBalance      xc.AmountBlockchain `json:"ton_balance"`
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}
var _ xc.TxInputWithMemo = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverTon,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverTon
}

func (input *TxInput) SetPublicKey(pk []byte) error {
	if len(pk) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid ed25519 public key size: %d", len(pk))
	}
	input.PublicKey = pk
	return nil
}
func (input *TxInput) SetPublicKeyFromStr(pkStr string) error {
	pkStr = strings.TrimPrefix(pkStr, "0x")
	pk, err := hex.DecodeString(pkStr)
	if err != nil {
		return fmt.Errorf("invalid hex: %v", err)
	}
	return input.SetPublicKey(pk)
}

func (input *TxInput) SetMemo(memo string) {
	input.Memo = memo
}
func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
}
func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	// TON doesn't have prioritization fees but we can map it to update the max fee reservation
	multipliedFee := multiplier.Mul(decimal.NewFromBigInt(input.EstimatedMaxFee.Int(), 0)).BigInt()
	input.EstimatedMaxFee = xc.AmountBlockchain(*multipliedFee)
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// different sequence means independence
	if evmOther, ok := other.(*TxInput); ok {
		return evmOther.Sequence != input.Sequence
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
