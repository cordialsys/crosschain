package sui

import (
	"bytes"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.BuilderSupportsFeePayer = &TxBuilder{}
var _ xcbuilder.Staking = &TxBuilder{}

func (txBuilder TxBuilder) SupportsFeePayer() xcbuilder.FeePayerType {
	return xcbuilder.FeePayerWithConflicts
}

// NewTxBuilder creates a Sui transaction builder.
func NewTxBuilder(asset *xc.ChainBaseConfig) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	feePayer, ok := args.GetFeePayer()
	if !ok {
		feePayer = args.GetFrom()
	}
	fromPubKey, ok := args.GetPublicKey()
	if !ok {
		return &Tx{}, errors.New("must set public key on TxInput for SUI")
	}
	local, ok := input.(*TxInput)
	if !ok {
		return nil, errors.New("xc.TxInput is not from a sui chain")
	}
	original := local
	defer func() { original.GasBudget = local.GasBudget }()
	if id, ok := args.GetTransactionID(); ok {
		if id == "" {
			return nil, fmt.Errorf("transaction ID must not be empty")
		}
		if local.TransactionID != "" && local.TransactionID != id {
			return nil, fmt.Errorf("transaction ID does not match TxInput")
		}
		// The argument is trusted application data, like the optional input setter.
		copyInput := local.Clone()
		copyInput.TransactionID = id
		local = copyInput
		input = local
	}
	requested, _ := args.GetContract()
	expected := string(requested)
	if expected == "" {
		expected = NativeCoin
	}
	expectedTag, err := parseCoinType(NormalizeCoinContract(expected))
	if err != nil {
		return nil, fmt.Errorf("invalid transfer coin type: %w", err)
	}
	expectedBytes, err := expectedTag.BcsSerialize()
	if err != nil {
		return nil, err
	}
	checkType := func(actual string) error {
		tag, err := parseCoinType(NormalizeCoinContract(actual))
		if err != nil {
			return err
		}
		encoded, err := tag.BcsSerialize()
		if err != nil {
			return err
		}
		if !bytes.Equal(expectedBytes, encoded) {
			return fmt.Errorf("transaction input coin type does not match requested transfer asset")
		}
		return nil
	}
	if local.AddressBalance != nil {
		if err := checkType(local.AddressBalance.CoinType); err != nil {
			return nil, err
		}
	}
	if local.paymentProtected() {
		if err := checkType(local.PaymentKit.CoinType); err != nil {
			return nil, err
		}
	}
	transferCoins := 0
	for _, coin := range local.Coins {
		if coin == nil {
			return nil, fmt.Errorf("nil transfer coin")
		}
		// Legacy inputs may also include GasCoin in Coins; newTransactionBase
		// removes that entry before building transfer commands.
		if local.AddressBalance == nil && coin.CoinObjectId.String() == local.GasCoin.CoinObjectId.String() {
			continue
		}
		transferCoins++
		if err := checkType(coin.CoinType); err != nil {
			return nil, err
		}
	}
	if local.AddressBalance == nil && transferCoins == 0 {
		// With no transfer coins the legacy builder uses GasCoin. Never allow a
		// missing token balance to turn a token request into a native SUI transfer.
		// Gas objects are intrinsically SUI; older inputs may omit CoinType.
		if err := checkType(NativeCoin); err != nil {
			return nil, fmt.Errorf("no coins available for requested transfer asset: %w", err)
		}
	}
	return txBuilder.NewTransfer(feePayer, fromPubKey, args.GetFrom(), args.GetTo(), args.GetAmount(), input)
}

// Data, inputs and commands that are common for all sui transactions
// Use SetInputsAndCommands to add custom behavior
type transactionBase struct {
	base bcs.TransactionData__V1
}

func (t *transactionBase) Build(inputs []bcs.CallArg, commands []bcs.Command) bcs.TransactionData__V1 {
	t.base.Value.Kind = &bcs.TransactionKind__ProgrammableTransaction{
		Value: bcs.ProgrammableTransaction{
			Inputs:   inputs,
			Commands: commands,
		},
	}

	return t.base
}

func (txBuilder TxBuilder) newTransactionBase(feePayer xc.Address, from xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (transactionBase, error) {
	var local_input *TxInput
	var ok bool
	if local_input, ok = input.(*TxInput); !ok {
		return transactionBase{}, errors.New("xc.TxInput is not from a sui chain")
	}

	if local_input.AddressBalance != nil {
		return txBuilder.newAddressTransactionBase(feePayer, from, amount, local_input)
	}

	fromData, err := HexToAddress(string(from))
	if err != nil {
		return transactionBase{}, fmt.Errorf("could not decode from address: %w", err)
	}
	feePayerData, err := HexToAddress(string(feePayer))
	if err != nil {
		return transactionBase{}, fmt.Errorf("could not decode fee payer address: %w", err)
	}

	gasObjectId, err := HexToObjectID(local_input.GasCoin.CoinObjectId.String())
	if err != nil {
		return transactionBase{}, fmt.Errorf("could not decode gas coin object id: %w", err)
	}
	local_input.GasCoin.Digest.Data()
	gasDigest, err := Base58ToObjectDigest(
		local_input.GasCoin.Digest.String(),
	)
	if err != nil {
		return transactionBase{}, fmt.Errorf("could not decode gas coin digest: %w", err)
	}
	gasVersion := local_input.GasCoin.Version.Uint64()

	local_input.ExcludeGasCoin()
	// Our gas budget should be the minimum of:
	//  - normal budget (2sui)
	//  - balance of the gas coin
	//  - total sui balance remainder after transfering `amount`.

	normal_budget := local_input.GasBudget
	gas_coin_balance := local_input.GasCoin.Balance.Uint64()
	total_remainder := gas_coin_balance
	if local_input.IsNativeTransfer() {
		if local_input.TotalBalance().Uint64() < amount.Uint64() {
			return transactionBase{}, fmt.Errorf("not enough funds to send after paying for sui gas: budget=%d tf=%d", local_input.GasBudget, amount.Uint64())
		}
		total_remainder = local_input.TotalBalance().Uint64() - amount.Uint64()
	}

	budget := normal_budget
	if gas_coin_balance < budget {
		budget = gas_coin_balance
	}
	if total_remainder < budget {
		budget = total_remainder
	}

	// lower the gas budget to whatever balance is on the gas coin.  no need to split it.
	local_input.GasBudget = budget

	// expires after current epoch
	expiration := bcs.TransactionExpiration__Epoch(local_input.CurrentEpoch)

	gasCoin := ObjectRef{
		Field0: gasObjectId,
		Field1: bcs.SequenceNumber(gasVersion),
		Field2: gasDigest,
	}

	// Common transaction
	tx := bcs.TransactionData__V1{
		Value: bcs.TransactionDataV1{
			GasData: bcs.GasData{
				Payment: []struct {
					Field0 bcs.ObjectID
					Field1 bcs.SequenceNumber
					Field2 bcs.ObjectDigest
				}{
					gasCoin,
				},
				Owner:  feePayerData,
				Price:  local_input.GasPrice,
				Budget: local_input.GasBudget,
			},
			Sender:     fromData,
			Expiration: &expiration,
			Kind:       nil,
		},
	}

	return transactionBase{
		tx,
	}, nil
}

// Create commands and inputs for proper gas spending and optimal transaction costs:
// 1. Split the gas coin if necessary
// 2. Merge coins if necessary
// 3. Set the primary coin when only one coin is provided
// Required for staking and transfer operations
func (txBuilder TxBuilder) prepareGasSplitAndMergeCommands(feePayer xc.Address, from xc.Address, input TxInput, amount xc.AmountBlockchain) (bcs.Argument, []bcs.Command, []bcs.CallArg, error) {

	if input.AddressBalance != nil {
		return txBuilder.prepareAddressBalanceCommands(feePayer, from, input, amount)
	}
	cmd_inputs := []bcs.CallArg{}
	commands := []bcs.Command{}
	var gasRemainderResult bcs.Argument
	// I. Split the gas coin if necessary
	// Check to see if we can afford the gas budget.
	if feePayer == from {
		if input.IsNativeTransfer() && len(input.Coins) > 0 {
			// Split off the remainder from gas budget
			remainder := input.GasCoin.Balance.Uint64() - input.GasBudget
			cmd_inputs = append(cmd_inputs, U64ToPure(remainder))

			commands = append(commands, &bcs.Command__SplitCoins{
				Field0: &bcs.Argument__GasCoin{},
				Field1: []bcs.Argument{
					ArgumentInput(uint16(0)),
				},
			})
			gasRemainderResult = ArgumentResult(uint16(len(commands) - 1))
		}
	}

	primaryCoinInput := gasRemainderResult

	// II. merge together rest of coins if needed
	if len(input.Coins) > 0 {
		// The first coin becomes our "primary coin"
		primaryCoinInput = ArgumentInput(uint16(len(cmd_inputs)))

		obj, err := CoinToObject(input.Coins[0])
		if err != nil {
			return nil, nil, nil, err
		}
		cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
			Value: obj,
		})

		merge_inputs := []bcs.Argument{}

		if gasRemainderResult != nil {
			merge_inputs = append(merge_inputs, ArgumentResult(uint16(len(commands)-1)))
		}

		for i, coin := range input.Coins[1:] {
			if i > MaxCoinObjects {
				break
			}
			obj, err := CoinToObject(coin)
			if err != nil {
				return nil, nil, nil, err
			}
			merge_inputs = append(merge_inputs, ArgumentInput(uint16(len(cmd_inputs))))

			cmd_inputs = append(cmd_inputs, &bcs.CallArg__Object{
				Value: obj,
			})
		}
		if len(merge_inputs) > 0 {
			commands = append(commands, &bcs.Command__MergeCoins{
				Field0: primaryCoinInput,
				Field1: merge_inputs,
			})
		}
	}

	if primaryCoinInput == nil {
		if feePayer == from {
			// if we only have one coin.. than the primary coin should be the gas coin
			primaryCoinInput = &bcs.Argument__GasCoin{}
		} else {
			return nil, nil, nil, fmt.Errorf("no coins to spend")
		}
	}

	return primaryCoinInput, commands, cmd_inputs, nil
}

// Old transfer interface
func (txBuilder TxBuilder) NewTransfer(feePayer xc.Address, fromPubKey []byte, from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	if amount.Sign() < 0 || amount.Int().BitLen() > 64 {
		return nil, fmt.Errorf("Sui transfer amount must fit uint64")
	}
	var local_input *TxInput
	var ok bool
	if local_input, ok = input.(*TxInput); !ok {
		return &Tx{}, errors.New("xc.TxInput is not from a sui chain")
	}
	if len(fromPubKey) == 0 {
		return &Tx{}, errors.New("must set public key on TxInput for SUI")
	}
	if local_input.TransactionID != "" {
		if err := txBuilder.validatePaymentKit(local_input); err != nil {
			return nil, err
		}
	}
	if !local_input.paymentProtected() && local_input.AddressBalance != nil {
		original := local_input
		local_input = local_input.coinOnlyFunding()
		input = local_input
		defer func() { original.GasBudget = local_input.GasBudget }()
	}

	txBase, err := txBuilder.newTransactionBase(
		feePayer,
		from,
		amount,
		input,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build transfer base: %w", err)
	}

	primaryCoinInput, commands, cmd_inputs, err := txBuilder.prepareGasSplitAndMergeCommands(feePayer, from, *local_input, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to create gas split and merge commands: %w", err)
	}

	// A freshly redeemed withdrawal is already exactly the requested amount.
	// Consume it directly: splitting would leave an unused (even if zero) Coin.
	transferCoin := primaryCoinInput
	_, freshWithdrawal := primaryCoinInput.(*bcs.Argument__Result)
	if local_input.AddressBalance == nil || !freshWithdrawal {
		// spend the primary coin by splitting `amt` from it
		commands = append(commands, &bcs.Command__SplitCoins{
			Field0: primaryCoinInput,
			Field1: []bcs.Argument{
				// the last input has the amount
				ArgumentInput(uint16(len(cmd_inputs))),
			},
		})
		cmd_inputs = append(cmd_inputs, U64ToPure(amount.Uint64()))

		transferCoin = ArgumentResult(uint16(len(commands) - 1))
	}

	if local_input.TransactionID != "" {
		cmd_inputs, commands, err = local_input.appendPayment(from, to, amount.Uint64(), transferCoin, cmd_inputs, commands)
		if err != nil {
			return nil, err
		}
	} else {
		// send the newly split object
		commands = append(commands, &bcs.Command__TransferObjects{
			Field0: []bcs.Argument{
				// Either the exact withdrawal or the split coin.
				transferCoin,
			},
			Field1: ArgumentInput(uint16(len(cmd_inputs))),
		})

		toPure, err := HexToPure(string(to))
		if err != nil {
			return nil, fmt.Errorf("failed to encode 'to': %w", err)
		}
		cmd_inputs = append(cmd_inputs, toPure)

	}

	xcTx := &Tx{
		Tx:         txBase.Build(cmd_inputs, commands),
		public_key: fromPubKey,
	}
	if feePayer != from {
		xcTx.extraFeePayer = feePayer
	}
	return xcTx, nil
}

func (txBuilder TxBuilder) NewNativeTransfer(feePayer xc.Address, fromPubKey []byte, from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	return txBuilder.NewTransfer(feePayer, fromPubKey, from, to, amount, input)
}
func (txBuilder TxBuilder) NewTokenTransfer(feePayer xc.Address, fromPubKey []byte, from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	// The token is already in the coins in the tx_input so txbuilding is the exact same.
	return txBuilder.NewTransfer(feePayer, fromPubKey, from, to, amount, input)
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	// SUI does not support memo
	return xc.MemoSupportNone
}
