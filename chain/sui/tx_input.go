package sui

import (
	"bytes"
	"sort"

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
	// Native Sui object that we can use to pay gas with
	GasCoin      types.Coin `json:"gas_coin,omitempty"`
	GasCoinOwner xc.Address `json:"gas_coin_owner,omitempty"`
	// All objects (native or token)
	Coins []*types.Coin `json:"coins,omitempty"`
	// current epoch
	CurrentEpoch uint64 `json:"current_epoch,omitempty"`
}

type CoinGetter interface {
	GetCoins() []*types.Coin
	GetGasCoin() *types.Coin
	GetCurrentEpoch() uint64
}

var _ xc.TxInput = &TxInput{}
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

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if suiOther, ok := other.(*TxInput); ok {
		// if epoch changed, means independence as one txInput expired
		if suiOther.CurrentEpoch != input.CurrentEpoch {
			return true
		}
		if input.coinsDisjoint(suiOther) {
			return true
		}
	}
	return
}
func CoinEqual(coin1 *types.Coin, coin2 *types.Coin) bool {
	return bytes.Equal(coin1.CoinObjectId.Data(), (coin2.CoinObjectId.Data())) &&
		bytes.Equal(coin1.Digest.Data(), coin2.Digest.Data())
}

func (input *TxInput) coinsDisjoint(other CoinGetter) (disjoint bool) {
	if CoinEqual(&input.GasCoin, other.GetGasCoin()) {
		return false
	}

	for _, coinOther := range other.GetCoins() {
		for _, coinInput := range input.Coins {
			// sui object id's are globally unique.
			if CoinEqual(coinOther, coinInput) {
				// not disjoint
				return false
			}
			// double check against the gas coin
			if CoinEqual(coinOther, &input.GasCoin) {
				return false
			}
			if CoinEqual(coinInput, other.GetGasCoin()) {
				return false
			}
		}
	}
	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if _, ok := other.(CoinGetter); !ok {
		return false
	}
	// all same sequence means no double send
	if suiOther, ok := other.(CoinGetter); ok {
		if input.coinsDisjoint(suiOther) && input.CurrentEpoch == suiOther.GetCurrentEpoch() {
			return false
		}
	} else {
		// can't tell, default false
		return false
	}
	// all either disjoint or different epoches - we're safe
	return true
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
