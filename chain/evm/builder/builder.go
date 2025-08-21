package builder

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/validation"
	"github.com/cordialsys/crosschain/chain/evm/abi/exit_request"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_batch_deposit"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

var DefaultMaxTipCapGwei uint64 = 5

// supports evm after london merge
type EvmTxBuilder struct {
}

// TxBuilder for EVM
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
	// Legacy bool
}

var _ xcbuilder.FullBuilder = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}
var _ xcbuilder.BuilderSupportsFeePayer = &TxBuilder{}
var _ xcbuilder.MultiTransfer = &TxBuilder{}

func NewEvmTxBuilder() *EvmTxBuilder {
	return &EvmTxBuilder{}
}

// NewTxBuilder creates a new EVM TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: asset,
	}, nil
}

func (txBuilder TxBuilder) SupportsFeePayer() xcbuilder.FeePayerType {
	return xcbuilder.FeePayerWithConflicts
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	return tx.NewTx(txBuilder.Asset, args, input.(*tx_input.TxInput), false)
}
func (txBuilder TxBuilder) MultiTransfer(args xcbuilder.MultiTransferArgs, input xc.MultiTransferInput) (xc.Tx, error) {
	return tx.NewMultiTx(txBuilder.Asset, args, &input.(*tx_input.MultiTransferInput).TxInput)
}

func (*EvmTxBuilder) BuildTxWithPayload(chain *xc.ChainBaseConfig, to xc.Address, value xc.AmountBlockchain, data []byte, inputRaw xc.TxInput) (xc.Tx, error) {
	address, err := address.FromHex(to)
	if err != nil {
		return nil, err
	}

	input := inputRaw.(*tx_input.TxInput)
	var chainId *big.Int = input.ChainId.Int()
	if input.ChainId.Uint64() == 0 {
		chainIdInt, _ := chain.ChainID.AsInt()
		chainId = new(big.Int).SetUint64(chainIdInt)
	}

	ethTx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainId,
		Nonce:     input.Nonce,
		GasTipCap: input.GasTipCap.Int(),
		GasFeeCap: input.GasFeeCap.Int(),
		Gas:       input.GasLimit,
		To:        &address,
		Value:     value.Int(),
		Data:      data,
	})

	return tx.NewCustomTx(input, chain, ethTx), nil
}

func GweiToWei(gwei uint64) xc.AmountBlockchain {
	bigGwei := big.NewInt(int64(gwei))

	ten := big.NewInt(10)
	nine := big.NewInt(9)
	factor := big.NewInt(0).Exp(ten, nine, nil)

	bigGwei.Mul(bigGwei, factor)
	return xc.AmountBlockchain(*bigGwei)
}

func (txBuilder TxBuilder) Stake(stakeArgs xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	switch input := input.(type) {
	case *tx_input.BatchDepositInput:
		evmBuilder := NewEvmTxBuilder()

		owner, ok := stakeArgs.GetStakeOwner()
		if !ok {
			owner = stakeArgs.GetFrom()
		}
		ownerAddr, err := address.FromHex(owner)
		if err != nil {
			return nil, err
		}
		ownerBz := ownerAddr.Bytes()
		withdrawCred := [32]byte{}
		copy(withdrawCred[32-len(ownerBz):], ownerBz)
		// set the credential type
		withdrawCred[0] = 1
		credentials := make([][]byte, len(input.PublicKeys))
		for i := range credentials {
			credentials[i] = withdrawCred[:]
		}
		data, err := stake_batch_deposit.Serialize(txBuilder.Asset, input.PublicKeys, credentials, input.Signatures)
		if err != nil {
			return nil, fmt.Errorf("invalid input for %T: %v", input, err)
		}
		contract := txBuilder.Asset.Staking.StakeContract
		tx, err := evmBuilder.BuildTxWithPayload(txBuilder.Asset, xc.Address(contract), stakeArgs.GetAmount(), data, &input.TxInput)
		if err != nil {
			return nil, fmt.Errorf("could not build tx for %T: %v", input, err)
		}
		return tx, nil
	default:
		return nil, fmt.Errorf("unsupported staking type %T", input)
	}
}
func (txBuilder TxBuilder) Unstake(stakeArgs xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	switch input := input.(type) {
	case *tx_input.ExitRequestInput:
		evmBuilder := NewEvmTxBuilder()

		count, err := validation.Count32EthChunks(stakeArgs.GetAmount())
		if err != nil {
			return nil, err
		}
		if int(count) > len(input.PublicKeys) {
			return nil, fmt.Errorf("need at least %d validators to unstake target amount, but there are only %d in eligible state", count, len(input.PublicKeys))
		}

		data, err := exit_request.Serialize(input.PublicKeys[:count])
		if err != nil {
			return nil, fmt.Errorf("invalid input for %T: %v", input, err)
		}
		contract := txBuilder.Asset.Staking.UnstakeContract
		zero := xc.NewAmountBlockchainFromUint64(0)
		tx, err := evmBuilder.BuildTxWithPayload(txBuilder.Asset, xc.Address(contract), zero, data, &input.TxInput)
		if err != nil {
			return nil, fmt.Errorf("could not build tx for %T: %v", input, err)
		}
		return tx, nil
	default:
		return nil, fmt.Errorf("unsupported unstaking type %T", input)
	}
}

func (txBuilder TxBuilder) Withdraw(stakeArgs xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("ethereum stakes are claimed automatically")
}
