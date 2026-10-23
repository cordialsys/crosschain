package sui

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/go-sui-sdk/v2/types"
)

// Variant methods must keep the enclosing input: an unstake consumes stake
// objects and gas, whereas a transfer or stake also consumes transfer coins.
func (input *StakingInput) IndependentOf(other xc.TxInput) bool {
	return inputsIndependent(input, other)
}
func (input *StakingInput) SafeFromDoubleSend(other xc.TxInput) bool {
	return inputsSafeFromDoubleSend(input, other)
}
func (input *UnstakingInput) IndependentOf(other xc.TxInput) bool {
	return inputsIndependent(input, other)
}
func (input *UnstakingInput) SafeFromDoubleSend(other xc.TxInput) bool {
	return inputsSafeFromDoubleSend(input, other)
}

func baseInput(input xc.TxInput) *TxInput {
	switch input := input.(type) {
	case *TxInput:
		return input
	case *StakingInput:
		if input != nil {
			return &input.TxInput
		}
	case *UnstakingInput:
		if input != nil {
			return &input.TxInput
		}
	}
	return nil
}

func protectedTransfer(input xc.TxInput) bool {
	tx, ok := input.(*TxInput)
	return ok && tx != nil && tx.paymentProtected()
}

func inputChain(input *TxInput) string {
	if input.PaymentKit != nil {
		return input.PaymentKit.ChainID
	}
	if input.AddressBalance != nil {
		return input.AddressBalance.ChainID
	}
	return ""
}

func comparableInputs(a, b *TxInput) bool {
	if a == nil || b == nil {
		return false
	}
	left, right := inputChain(a), inputChain(b)
	// Legacy inputs have no chain ID. The interface assumes the same chain and
	// address, but reject an explicit network mismatch when metadata is available.
	return left == "" || right == "" || left == right
}

func inputsIndependent(current, previous xc.TxInput) bool {
	a, b := baseInput(current), baseInput(previous)
	if !comparableInputs(a, b) {
		return false
	}
	if protectedTransfer(current) && protectedTransfer(previous) && a.TransactionID == b.TransactionID {
		return false
	}
	// Every Sui builder encodes CurrentEpoch as the transaction's maximum
	// execution epoch. Only a later epoch proves that the older input expired.
	// This permits queuing a different transaction; it does NOT prove retry safety.
	if a.CurrentEpoch > b.CurrentEpoch {
		return true
	}
	return !resourcesOverlap(inputResources(current, false), inputResources(previous, false))
}

func inputsSafeFromDoubleSend(current, previous xc.TxInput) bool {
	a, b := baseInput(current), baseInput(previous)
	if !comparableInputs(a, b) {
		return false
	}
	if protectedTransfer(current) && protectedTransfer(previous) {
		left, right := a.PaymentKit, b.PaymentKit
		// An ID names an immutable transfer. Retention is bounded and assumes a
		// trusted registry administrator, who can change its configuration.
		if a.TransactionID == b.TransactionID && left.ChainID == right.ChainID &&
			left.PackageID == right.PackageID && left.RegistryID == right.RegistryID &&
			left.Sender == right.Sender && left.CoinType == right.CoinType &&
			a.CurrentEpoch >= b.CurrentEpoch && a.CurrentEpoch-b.CurrentEpoch < min(left.RetentionEpochs, right.RetentionEpochs) {
			return true
		}
	}
	// Expiration cannot establish that an old attempt never executed. Apart from
	// Payment Kit, require an object version that BOTH attempts actually consume.
	return resourcesOverlap(inputResources(current, true), inputResources(previous, true))
}

type ownedResource struct {
	id      string
	version uint64
	digest  string
}

func resourcesOverlap(a, b map[ownedResource]struct{}) bool {
	for ref := range a {
		if _, ok := b[ref]; ok {
			return true
		}
	}
	return false
}

// Queuing considers every candidate resource (conservative false conflicts are
// harmless). Retry safety uses only resources guaranteed to be consumed. In
// particular, a listed sponsor gas coin may be replaced by address-funded gas.
func inputResources(input xc.TxInput, guaranteed bool) map[ownedResource]struct{} {
	refs := map[ownedResource]struct{}{}
	base := baseInput(input)
	if base == nil {
		return refs
	}
	addCoins := func(coins ...*types.Coin) {
		for _, coin := range coins {
			if coin != nil && len(coin.CoinObjectId.Data()) != 0 && len(coin.Digest) != 0 {
				refs[ownedResource{coin.CoinObjectId.String(), coin.Version.Uint64(), coin.Digest.String()}] = struct{}{}
			}
		}
	}
	unstake, isUnstake := input.(*UnstakingInput)
	if !guaranteed {
		addCoins(&base.GasCoin)
		addCoins(base.Coins...)
		if base.AddressBalance != nil {
			addCoins(base.AddressBalance.GasCoins...)
		}
	} else if base.AddressBalance == nil {
		addCoins(&base.GasCoin)
		if !isUnstake {
			// The legacy builder bounds its merge list. Do not infer conflicts from
			// entries past the supported coin-selection limit.
			count := 0
			for _, coin := range base.Coins {
				if coin == nil || coin.CoinObjectId.String() == base.GasCoin.CoinObjectId.String() {
					continue
				}
				if count == MaxCoinObjects {
					break
				}
				addCoins(coin)
				count++
			}
		}
	} else {
		coins := base.addressTransferCoins()
		if !isUnstake {
			// Both address-gas and coin-gas plans consume these coins, including
			// mixed funding and native gas smashing.
			addCoins(coins...)
		}
		gasCoins := base.AddressBalance.GasCoins
		if base.IsNativeTransfer() && base.GasCoinOwner == base.AddressBalance.Sender {
			gasCoins = coins
		}
		balance, err := coinSum(gasCoins)
		if err == nil && base.AddressBalance.GasBalance <= balance {
			addCoins(gasCoins...)
		}
		// Older software could spend address funds without Payment Kit. Do not
		// retroactively assume its optional gas coins were consumed, even though
		// new unprotected builds restrict funding to coins.
	}
	if isUnstake {
		stakes := append([]Stake(nil), unstake.StakesToUnstake...)
		balance := unstake.StakeToSplit.GetBalance()
		if !balance.IsZero() {
			stakes = append(stakes, unstake.StakeToSplit)
		}
		for _, stake := range stakes {
			if stake.ObjectId != "" && stake.Digest != "" {
				refs[ownedResource{stake.ObjectId, stake.Version, stake.Digest}] = struct{}{}
			}
		}
	}
	return refs
}
