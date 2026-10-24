package sui

import (
	"bytes"
	"slices"
	"sort"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/shopspring/decimal"
)

// Tx for Template
type TxInput struct {
	xc.TxInputEnvelope
	GasBudget uint64 `json:"gas_budget,omitempty"`
	GasPrice  uint64 `json:"gas_price,omitempty"`
	// True when fetching without an ID reserved different fees for two possible
	// funding paths. Such a budget is a ceiling, not an exact send-max deduction.
	GasBudgetUpperBound bool `json:"gas_budget_upper_bound,omitempty"`
	// Native Sui object that we can use to pay gas with
	GasCoin      types.Coin `json:"gas_coin,omitempty"`
	GasCoinOwner xc.Address `json:"gas_coin_owner,omitempty"`
	// All objects (native or token)
	Coins []*types.Coin `json:"coins,omitempty"`
	// Optional address-balance funding. Omitted inputs use the legacy builder.
	AddressBalance *AddressBalanceInput `json:"address_balance,omitempty"`
	// Application-supplied identifier; clients leave this unset.
	TransactionID string           `json:"transaction_id,omitempty"`
	PaymentKit    *PaymentKitInput `json:"payment_kit,omitempty"`
	// current epoch
	CurrentEpoch uint64 `json:"current_epoch,omitempty"`
}

// AddressBalanceInput adds funding sources without changing legacy coin fields.
// Balances are decimal strings on the wire to preserve uint64 precision.
type AddressBalanceInput struct {
	Sender     xc.Address `json:"sender"`
	CoinType   string     `json:"coin_type"`
	Balance    uint64     `json:"balance,string"`
	GasBalance uint64     `json:"gas_balance,string"`
	// Includes GasCoin and any additional SUI coins owned by GasCoinOwner.
	GasCoins []*types.Coin `json:"gas_coins,omitempty"`
	ChainID  string        `json:"chain_id"`
	// Generated once when fetching inputs; offline rebuilds keep the same nonce.
	Nonce uint32 `json:"nonce"`
}

func (input *TxInput) GetAddressBalanceInput() *AddressBalanceInput { return input.AddressBalance }

type CoinGetter interface {
	GetCoins() []*types.Coin
	GetGasCoin() *types.Coin
	GetCurrentEpoch() uint64
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithTransactionID = &TxInput{}

func (input *TxInput) SetTransactionID(id string) { input.TransactionID = id }

// Clone copies mutable funding metadata so planning/building never changes
// another candidate's slice or balances. Individual coin records are read-only.
func (input *TxInput) Clone() *TxInput {
	copyInput := *input
	copyInput.Coins = slices.Clone(input.Coins)
	if input.AddressBalance != nil {
		balance := *input.AddressBalance
		balance.GasCoins = slices.Clone(balance.GasCoins)
		copyInput.AddressBalance = &balance
	}
	if input.PaymentKit != nil {
		kit := *input.PaymentKit
		copyInput.PaymentKit = &kit
	}
	return &copyInput
}

var _ CoinGetter = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakingInput{})
	registry.RegisterTxVariantInput(&UnstakingInput{})
}

func (input *TxInput) GetCoins() []*types.Coin {
	return input.Coins
}
func (input *TxInput) GetGasCoin() *types.Coin {
	return &input.GasCoin
}

func (input *TxInput) GetCurrentEpoch() uint64 {
	return input.CurrentEpoch
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverSui
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	multipliedGasPrice := multiplier.Mul(decimal.NewFromInt(int64(input.GasPrice)))
	multipliedGasBudget := multiplier.Mul(decimal.NewFromInt(int64(input.GasBudget)))
	input.GasPrice = multipliedGasPrice.BigInt().Uint64()
	input.GasBudget = multipliedGasBudget.BigInt().Uint64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	maxBudget := xc.NewAmountBlockchainFromUint64(input.GasBudget)
	return maxBudget, ""
}

func (input *TxInput) IndependentOf(other xc.TxInput) bool {
	return inputsIndependent(input, other)
}
func (input *TxInput) IsFeeLimitAccurate() bool {
	return !input.GasBudgetUpperBound
}
func CoinEqual(coin1 *types.Coin, coin2 *types.Coin) bool {
	return bytes.Equal(coin1.CoinObjectId.Data(), (coin2.CoinObjectId.Data())) &&
		coin1.Version.Uint64() == coin2.Version.Uint64() &&
		bytes.Equal(coin1.Digest.Data(), coin2.Digest.Data())
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) bool {
	return inputsSafeFromDoubleSend(input, other)
}

func SortCoins(coins []*types.Coin) {
	sort.Slice(coins, func(i, j int) bool {
		return coins[i].Balance.Decimal().Cmp(coins[j].Balance.Decimal()) > 0
	})
}

func (input *TxInput) ExcludeGasCoin() {
	for i, coin := range input.Coins {
		if coin.CoinObjectId.String() == input.GasCoin.CoinObjectId.String() {
			// drop it
			input.Coins = append(input.Coins[:i], input.Coins[i+1:]...)
			break
		}
	}
}

func (input *TxInput) TotalBalance() xc.AmountBlockchain {
	if a := input.AddressBalance; a != nil {
		total := xc.NewAmountBlockchainFromUint64(a.Balance)
		for _, coin := range input.addressTransferCoins() {
			v := xc.NewAmountBlockchainFromUint64(coin.Balance.Uint64())
			total = total.Add(&v)
		}
		return total
	}

	amount := xc.NewAmountBlockchainFromUint64(0)
	coinType := ""
	for _, coin := range input.Coins {
		coinType = coin.CoinType
		coinBal := xc.NewAmountBlockchainFromUint64(coin.Balance.Uint64())
		amount = amount.Add(&coinBal)
	}
	// add gas coin if it's same type
	if coinType == "" || coinType == input.GasCoin.CoinType {
		coinBal := xc.NewAmountBlockchainFromUint64(input.GasCoin.Balance.Uint64())
		amount = amount.Add(&coinBal)
	}
	return amount
}

// Sort coins in place from highest to lowest
func (input *TxInput) SortCoins() {
	SortCoins(input.Coins)
}

func (input *TxInput) IsNativeTransfer() bool {
	if input.AddressBalance != nil {
		parts := strings.Split(NormalizeCoinContract(input.AddressBalance.CoinType), "::")
		return len(parts) == 3 && strings.TrimLeft(strings.TrimPrefix(parts[0], "0x"), "0") == "2" && parts[1] == "sui" && parts[2] == "SUI"
	}
	if len(input.Coins) > 0 && input.Coins[0].CoinType != input.GasCoin.CoinType {
		return false
	}
	return true
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverSui,
		},
	}
}
