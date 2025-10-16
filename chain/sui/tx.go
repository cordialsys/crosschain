package sui

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/blake2b"
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

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
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

func (input *TxInput) coinsDisjoint(other *TxInput) (disjoint bool) {
	if CoinEqual(&input.GasCoin, &other.GasCoin) {
		return false
	}

	for _, coinOther := range other.Coins {
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
			if CoinEqual(coinInput, &other.GasCoin) {
				return false
			}
		}
	}
	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if !xc.IsTypeOf(other, input) {
		return false
	}
	// all same sequence means no double send
	if suiOther, ok := other.(*TxInput); ok {
		if input.coinsDisjoint(suiOther) && input.CurrentEpoch == suiOther.CurrentEpoch {
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

type Tx struct {
	// Input      TxInput
	signatures    [][]byte
	public_key    []byte
	Tx            bcs.TransactionData__V1
	extraFeePayer xc.Address
}

var _ xc.Tx = &Tx{}
var _ xc.TxWithMetadata = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	typeTag := "TransactionData::"
	bz, err := tx.Serialize()
	if err != nil {
		panic(err)
	}
	tohash := append([]byte(typeTag), bz...)
	hash := blake2b.Sum256(tohash)
	hash_b58 := base58.Encode(hash[:])
	return xc.TxHash(hash_b58)
}
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	bytes, err := tx.Serialize()
	// 0 = transaction data, 0 = V0 intent version, 0 = sui
	// https://github.com/MystenLabs/sui/blob/a78b9e3f8a212924848f540da5a2587526525853/sdk/typescript/src/utils/intent.ts#L26
	intent := []byte{0, 0, 0}
	msg := append(intent, bytes...)
	hash := blake2b.Sum256(msg)

	if err != nil {
		return []*xc.SignatureRequest{}, err
	}
	if tx.extraFeePayer != "" {
		return []*xc.SignatureRequest{
			// the order doesn't matter for SUI
			xc.NewSignatureRequest(hash[:]),
			xc.NewSignatureRequest(hash[:], tx.extraFeePayer),
		}, nil
	} else {
		return []*xc.SignatureRequest{xc.NewSignatureRequest(hash[:])}, nil
	}
}
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	for _, sig := range signatures {
		// sui expects signature to be {0, signature, public_key}
		sui_sig := []byte{0}
		sui_sig = append(sui_sig, sig.Signature...)
		sui_sig = append(sui_sig, sig.PublicKey...)
		tx.signatures = append(tx.signatures, sui_sig)
	}
	return nil
}
func (tx Tx) Serialize() ([]byte, error) {
	bytes, err := tx.Tx.BcsSerialize()
	if err != nil {
		return bytes, err
	}
	return bytes, nil
}

type BroadcastMetadata struct {
	// SUI rpc requires signatures to be in separate field
	Signatures [][]byte `json:"signatures"`
}

func (tx Tx) GetMetadata() ([]byte, error) {
	metadata := BroadcastMetadata{
		Signatures: tx.signatures,
	}
	metadataBz, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return metadataBz, nil
}

type StakingInput struct {
	TxInput
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*StakingInput) Staking() {}
func (*StakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverSui, string(xc.Native))
}

type Stake struct {
	Principal xc.AmountBlockchain `json:"principal"`
	Rewards   xc.AmountBlockchain `json:"rewards"`
	ObjectId  string              `json:"object_id"`
	Version   uint64              `json:"version"`
	Digest    string              `json:"digest"`
	State     xclient.StakeState  `json:"state"`
	Validator string              `json:"validator"`
}

func (s Stake) GetBalance() xc.AmountBlockchain {
	return s.Principal.Add(&s.Rewards)
}

// Try to split the stake into specified and remaining amount
//
// Sui split's are based on Principal amounts, not Principal + Rewards. We have to calculate
// principal-to-balance ratio, and check if amount * principalRatio can be split off
//
// Returns (remainingPrincipalAmount, true) if the split is possible
func (s Stake) TrySplit(amount xc.AmountBlockchain, minStakeAmount xc.AmountBlockchain, decimals int32) (xc.AmountBlockchain, bool) {
	// at least 2 * minStakeAmount is required for a valid SUI stake split to cover min amount
	// in both stake objects
	two := xc.NewAmountBlockchainFromUint64(2)
	doubleMinAmount := minStakeAmount.Mul(&two)
	if s.Principal.Cmp(&doubleMinAmount) < 0 {
		return xc.AmountBlockchain{}, false
	}

	// calculate principal/balance ratio
	principalDecimal := s.Principal.ToHuman(decimals).Decimal()
	balance := s.GetBalance()
	balanceDeciamal := balance.ToHuman(decimals).Decimal()
	principalRatio := principalDecimal.Div(balanceDeciamal)

	// calculate required principal part to split
	// it cannot be greater than 's.Principal - minStakeAmount'
	amountDecimal := amount.ToHuman(decimals).Decimal()
	amountPrincipal := amountDecimal.Mul(principalRatio)
	minStakeAmountDecimal := minStakeAmount.ToHuman(decimals).Decimal()
	maxStakeAmount := s.Principal.Sub(&minStakeAmount)
	maxStakeAmountDecimal := maxStakeAmount.ToHuman(decimals).Decimal()
	if amountPrincipal.Cmp(minStakeAmountDecimal) < 0 || amountPrincipal.Cmp(maxStakeAmountDecimal) == 1 {
		return xc.NewAmountBlockchainFromUint64(0), false
	}

	remainingPrincipal := principalDecimal.Sub(amountPrincipal)
	hrRemaining, err := xc.NewAmountHumanReadableFromStr(remainingPrincipal.String())
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), false
	}
	return hrRemaining.ToBlockchain(decimals), true
}

type UnstakingInput struct {
	TxInput
	// Stakes that can be fully unstaked via `request_withdraw_stake`
	StakesToUnstake []Stake `json:"stakes_to_unstake"`
	// Stake to split to split for remaining amount
	StakeToSplit Stake `json:"stake_to_split"`
	// Amount to split from staking account
	SplitAmount xc.AmountBlockchain `json:"split_amount"`
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*UnstakingInput) Unstaking() {}
func (*UnstakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverSui, string(xc.Native))
}
