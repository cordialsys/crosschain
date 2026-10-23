package sui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"github.com/stretchr/testify/require"
)

func conflictCoin(t *testing.T, id, balance uint64) *types.Coin {
	t.Helper()
	coin := &types.Coin{}
	require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"coinObjectId":"0x%064x","version":"1","digest":"11111111111111111111111111111111","balance":"%d","coinType":%q}`, id, balance, NativeCoin)), coin))
	return coin
}

func conflictInput(t *testing.T, id string) *TxInput {
	t.Helper()
	input := protectedInput(id)
	input.GasPrice, input.GasBudget = 1, 1
	input.PaymentKit.PackageID = "0x" + strings.Repeat("44", 32)
	input.PaymentKit.RegistryID = "0x" + strings.Repeat("55", 32)
	input.PaymentKit.InitialSharedVersion = 1
	input.PaymentKit.ChainID = "11111111111111111111111111111111"
	input.GasCoinOwner = input.PaymentKit.Sender
	input.AddressBalance = &AddressBalanceInput{Sender: input.GasCoinOwner, CoinType: NativeCoin, Balance: 100, GasBalance: 100, ChainID: input.PaymentKit.ChainID}
	return input
}

func conflictBuilderConfig() *xc.ChainBaseConfig {
	cfg := xc.NewChainConfig(xc.SUI).Base()
	cfg.SuiPaymentKit = &xc.SuiPaymentKitConfig{PackageID: "0x" + strings.Repeat("44", 32), RegistryID: "0x" + strings.Repeat("55", 32)}
	return cfg
}

func transactionResources(tx *Tx) map[ownedResource]struct{} {
	refs := map[ownedResource]struct{}{}
	add := func(id bcs.ObjectID, version bcs.SequenceNumber, digest bcs.ObjectDigest) {
		refs[ownedResource{fmt.Sprintf("0x%x", id.Value), uint64(version), base58.Encode(digest.Value)}] = struct{}{}
	}
	for _, gas := range tx.Tx.Value.GasData.Payment {
		add(gas.Field0, gas.Field1, gas.Field2)
	}
	ptb := tx.Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value
	for _, arg := range ptb.Inputs {
		if object, ok := arg.(*bcs.CallArg__Object); ok {
			if owned, ok := object.Value.(*bcs.ObjectArg__ImmOrOwnedObject); ok {
				add(owned.Field0, owned.Field1, owned.Field2)
			}
		}
	}
	return refs
}

func TestConflictProofMatchesBuiltTransfers(t *testing.T) {
	for _, sponsored := range []bool{false, true} {
		for _, gasBalance := range []uint64{0, 100} {
			for _, id := range []string{"", "transfer"} {
				t.Run(fmt.Sprintf("sponsor=%t/addressgas=%d/id=%s", sponsored, gasBalance, id), func(t *testing.T) {
					input := conflictInput(t, id)
					input.Coins = []*types.Coin{conflictCoin(t, 1, 20)}
					input.GasCoin = *conflictCoin(t, 2, 20)
					input.AddressBalance.GasCoins = []*types.Coin{&input.GasCoin}
					input.AddressBalance.Balance, input.AddressBalance.GasBalance = gasBalance, gasBalance
					if sponsored {
						input.GasCoinOwner = xc.Address("0x" + strings.Repeat("33", 32))
					}
					b, err := NewTxBuilder(conflictBuilderConfig())
					require.NoError(t, err)
					tx, err := b.NewTransfer(input.GasCoinOwner, make([]byte, 32), input.AddressBalance.Sender, input.AddressBalance.Sender, xc.NewAmountBlockchainFromUint64(10), input)
					require.NoError(t, err)
					actual := transactionResources(tx.(*Tx))
					for ref := range inputResources(input, true) {
						require.Contains(t, actual, ref, "retry proof must reference an object actually consumed")
					}
					for ref := range actual {
						require.Contains(t, inputResources(input, false), ref, "queuing must consider every consumed object")
					}
					if id == "" {
						require.NotEmpty(t, tx.(*Tx).Tx.Value.GasData.Payment)
						for _, arg := range tx.(*Tx).Tx.Value.Kind.(*bcs.TransactionKind__ProgrammableTransaction).Value.Inputs {
							_, withdrawal := arg.(*bcs.CallArg__FundsWithdrawal)
							require.False(t, withdrawal)
						}
					}
				})
			}
		}
	}
}

func TestRetrySafetyRequiresConsumedObjectOrPaymentID(t *testing.T) {
	old, next := conflictInput(t, ""), conflictInput(t, "")
	// Missing gas objects must not match one another, including after expiration.
	next.CurrentEpoch++
	require.False(t, next.SafeFromDoubleSend(old))
	require.True(t, next.IndependentOf(old))
	// Coin-only inputs must share an exact version, not merely an object ID.
	old.AddressBalance, next.AddressBalance = nil, nil
	old.GasCoin, next.GasCoin = *conflictCoin(t, 1, 20), *conflictCoin(t, 2, 20)
	require.False(t, next.SafeFromDoubleSend(old))
	next.GasCoin = old.GasCoin
	require.True(t, next.SafeFromDoubleSend(old))
	next.CurrentEpoch = old.CurrentEpoch
	require.False(t, next.IndependentOf(old))
	// Gas-versus-transfer overlap is significant even with an empty coin slice.
	next.GasCoin = *conflictCoin(t, 2, 20)
	next.Coins = []*types.Coin{&old.GasCoin}
	require.True(t, next.SafeFromDoubleSend(old))
	require.False(t, next.IndependentOf(old))
	require.True(t, old.SafeFromDoubleSend(next))
	require.False(t, old.IndependentOf(next))

	old, next = conflictInput(t, "old"), conflictInput(t, "next")
	for _, in := range []*TxInput{old, next} {
		in.GasCoinOwner = xc.Address("0x" + strings.Repeat("33", 32))
		in.GasCoin = *conflictCoin(t, 3, 20)
		in.AddressBalance.GasCoins = []*types.Coin{&in.GasCoin}
	}
	// Both transfers use address-funded sponsor gas; the shared coin is unused.
	require.False(t, next.SafeFromDoubleSend(old))
	// Queuing is deliberately conservative about optional references.
	require.False(t, next.IndependentOf(old))
	// Neither stripping an ID from a cached input nor crossing epochs proves safety.
	old.TransactionID, next.TransactionID = "", ""
	next.CurrentEpoch++
	require.False(t, next.SafeFromDoubleSend(old))

	old, next = conflictInput(t, "same"), conflictInput(t, "same")
	require.True(t, next.SafeFromDoubleSend(old))
	next.CurrentEpoch += old.PaymentKit.RetentionEpochs
	require.False(t, next.SafeFromDoubleSend(old))
	require.False(t, next.IndependentOf(old))
}

func TestStakingConflictResources(t *testing.T) {
	base := conflictInput(t, "same")
	// Even inherited Payment Kit metadata cannot protect a stake: no registry call.
	a, b := &StakingInput{TxInput: *base}, &StakingInput{TxInput: *base}
	require.False(t, a.SafeFromDoubleSend(b))
	stake := Stake{Principal: xc.NewAmountBlockchainFromUint64(10), ObjectId: fmt.Sprintf("0x%064x", 9), Version: 1, Digest: "11111111111111111111111111111111"}
	u, v := &UnstakingInput{TxInput: *base, StakesToUnstake: []Stake{stake}}, &UnstakingInput{TxInput: *base, StakesToUnstake: []Stake{stake}}
	require.False(t, u.IndependentOf(v))
	require.True(t, u.SafeFromDoubleSend(v))
	u.CurrentEpoch++
	require.True(t, u.IndependentOf(v))
	require.True(t, u.SafeFromDoubleSend(v))
	u.StakesToUnstake = nil
	require.False(t, u.SafeFromDoubleSend(v))
	// Unstaking never consumes transfer coins merely listed in the base input.
	u.Coins, v.Coins = []*types.Coin{conflictCoin(t, 1, 20)}, []*types.Coin{conflictCoin(t, 1, 20)}
	u.GasCoinOwner, v.GasCoinOwner = "sponsor", "sponsor"
	require.False(t, u.SafeFromDoubleSend(v))
}

func TestAddressFundingRequiresProtectedTransfer(t *testing.T) {
	b, err := NewTxBuilder(conflictBuilderConfig())
	require.NoError(t, err)
	input := conflictInput(t, "")
	_, err = b.NewTransfer(input.GasCoinOwner, make([]byte, 32), input.AddressBalance.Sender, input.AddressBalance.Sender, xc.NewAmountBlockchainFromUint64(10), input)
	require.Error(t, err)
	input.SetTransactionID("protected")
	tx, err := b.NewTransfer(input.GasCoinOwner, make([]byte, 32), input.AddressBalance.Sender, input.AddressBalance.Sender, xc.NewAmountBlockchainFromUint64(10), input)
	require.NoError(t, err)
	require.Empty(t, tx.(*Tx).Tx.Value.GasData.Payment)
	// Staking cannot enable address funding just by inheriting an ID and metadata.
	args, err := builder.NewStakeArgs(xc.SUI, input.AddressBalance.Sender, builder.OptionPublicKey(make([]byte, 32)), builder.OptionValidator(string(input.AddressBalance.Sender)), builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(10)))
	require.NoError(t, err)
	_, err = b.Stake(args, &StakingInput{TxInput: *input})
	require.Error(t, err)
	_, err = b.Unstake(args, &UnstakingInput{TxInput: *input})
	require.Error(t, err)
}
