package builder

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	"github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	compute_budget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
)

// TxBuilder for Solana
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullBuilder = &TxBuilder{}

// Solana driver supports fee payer
var _ xcbuilder.BuilderSupportsFeePayer = &TxBuilder{}

func (txBuilder TxBuilder) SupportsFeePayer() {}

type TxInput = tx_input.TxInput

// Max number of token transfers we can fit in a solana transaction,
// when there's also a create ATA included.
const MaxTokenTransfers = 20
const MaxAccountUnstakes = 20
const MaxAccountWithdraws = 20

// NewTxBuilder creates a new Solana TxBuilder
func NewTxBuilder(asset *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {

	feePayer, ok := args.GetFeePayer()
	if !ok {
		feePayer = args.GetFrom()
	}

	if contract, ok := args.GetContract(); ok {
		decimals, ok := args.GetDecimals()
		if !ok {
			return nil, fmt.Errorf("cannot send solana token transfer without knowing the decimals")
		}
		return txBuilder.NewTokenTransfer(feePayer, args.GetFrom(), args.GetTo(), args.GetAmount(), contract, decimals, input)
	} else {
		return txBuilder.NewNativeTransfer(feePayer, args.GetFrom(), args.GetTo(), args.GetAmount(), input)
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(feePayer xc.Address, from xc.Address, to xc.Address, amount xc.AmountBlockchain, txInput xc.TxInput) (xc.Tx, error) {
	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}
	accountTo, err := solana.PublicKeyFromBase58(string(to))
	if err != nil {
		return nil, err
	}
	input := txInput.(*TxInput)

	instructions := []solana.Instruction{
		system.NewTransferInstruction(
			amount.Uint64(),
			accountFrom,
			accountTo,
		).Build(),
	}
	priorityFee := input.GetPrioritizationFee()
	if priorityFee > 0 {
		instructions = append(instructions,
			compute_budget.NewSetComputeUnitPriceInstruction(priorityFee).Build(),
		)
	}

	return txBuilder.buildSolanaTx(feePayer, from, instructions, input)
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(feePayer xc.Address, from xc.Address, to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress, decimals int, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	if contract == "" {
		return nil, errors.New("asset does not have a contract")
	}

	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}

	accountContract, err := solana.PublicKeyFromBase58(string(contract))
	if err != nil {
		return nil, err
	}
	accountTo, err := solana.PublicKeyFromBase58(string(to))
	if err != nil {
		return nil, err
	}

	ataFromStr, err := types.FindAssociatedTokenAddress(string(from), string(contract), solana.PublicKey(txInput.TokenProgram))
	if err != nil {
		return nil, err
	}
	ataFrom := solana.MustPublicKeyFromBase58(ataFromStr)
	if len(txInput.SourceTokenAccounts) > 0 {
		ataFrom = txInput.SourceTokenAccounts[0].Account
	}

	ataTo := accountTo
	if !txInput.ToIsATA {
		ataToStr, err := types.FindAssociatedTokenAddress(string(to), string(contract), solana.PublicKey(txInput.TokenProgram))
		if err != nil {
			return nil, err
		}
		ataTo = solana.MustPublicKeyFromBase58(ataToStr)
	}

	// Temporarily adjust the backend library to use a different program ID.
	// This is to support token2022 and potential other future variants.
	originalTokenId := token.ProgramID
	defer func() {
		token.ProgramID = originalTokenId
	}()
	if !txInput.TokenProgram.IsZero() && !txInput.TokenProgram.Equals(originalTokenId) {
		token.ProgramID = txInput.TokenProgram
	}

	instructions := []solana.Instruction{}
	if txInput.ShouldCreateATA {
		// fee payer should pay for the ATA creation, if used.
		accountCreatorPayer := accountFrom
		if feePayer != from {
			accountFeePayer, err := solana.PublicKeyFromBase58(string(feePayer))
			if err != nil {
				return nil, err
			}
			accountCreatorPayer = accountFeePayer
		}
		createAta := ata.NewCreateInstruction(
			accountCreatorPayer,
			accountTo,
			accountContract,
		).Build()
		// Adjust the ata-create-account arguments:
		// index 1 - associated token account
		// index 5 - token program
		createAta.Impl.(ata.Create).AccountMetaSlice[1].PublicKey = ataTo
		createAta.Impl.(ata.Create).AccountMetaSlice[5].PublicKey = txInput.TokenProgram
		instructions = append(instructions,
			createAta,
		)
	}
	if len(txInput.SourceTokenAccounts) <= 1 {
		// just send 1 instruction using the single ATA
		instructions = append(instructions,
			token.NewTransferCheckedInstruction(
				amount.Uint64(),
				uint8(decimals),
				ataFrom,
				accountContract,
				ataTo,
				accountFrom,
				[]solana.PublicKey{},
			).Build(),
		)
	} else {
		// Sometimes tokens can get put into any number of auxiliary accounts.
		// So we need to spend them like UTXO. Here we'll just send a solana
		// instruction for each one until we've reached the target balance.
		zero := xc.NewAmountBlockchainFromUint64(0)
		remainingBalanceToSend := amount
		for _, tokenAcc := range txInput.SourceTokenAccounts {
			amountToSend := remainingBalanceToSend
			if tokenAcc.Balance.Cmp(&remainingBalanceToSend) < 0 {
				// Send everything in the token account
				amountToSend = tokenAcc.Balance
			}
			amountToSendUint := amountToSend.Uint64()
			instructions = append(instructions,
				token.NewTransferCheckedInstruction(
					amountToSendUint,
					uint8(decimals),
					tokenAcc.Account,
					accountContract,
					ataTo,
					accountFrom,
					[]solana.PublicKey{},
				).Build(),
			)
			remainingBalanceToSend = remainingBalanceToSend.Sub(&amountToSend)
			if remainingBalanceToSend.Cmp(&zero) <= 0 {
				// we've spent enough from source accounts to meet target balance
				break
			}
			if len(instructions) > MaxTokenTransfers {
				return nil, errors.New("cannot send total amount in single tx, try sending smaller amount")
			}
		}
		if remainingBalanceToSend.Cmp(&zero) > 0 {
			return nil, errors.New("cannot send requested amount in single tx, try sending smaller amount")
		}
	}

	// add priority fee last
	priorityFee := txInput.GetPrioritizationFee()
	if priorityFee > 0 {
		instructions = append(instructions,
			compute_budget.NewSetComputeUnitPriceInstruction(priorityFee).Build(),
		)
	}

	return txBuilder.buildSolanaTx(feePayer, from, instructions, txInput)
}

func (txBuilder TxBuilder) buildSolanaTx(feePayer xc.Address, from xc.Address, instructions []solana.Instruction, txInput *TxInput) (*tx.Tx, error) {
	accountFeePayer, err := solana.PublicKeyFromBase58(string(feePayer))
	if err != nil {
		return nil, err
	}

	tx1, err := solana.NewTransaction(
		instructions,
		txInput.RecentBlockHash,
		solana.TransactionPayer(accountFeePayer),
	)
	if err != nil {
		return nil, err
	}
	tx := &tx.Tx{
		SolTx: tx1,
	}
	if feePayer != from {
		tx.SetExtraFeePayerSigner(feePayer)
	}
	return tx, nil
}

func (txBuilder TxBuilder) BuildUnwrapEverythingTx(from xc.Address, to xc.Address, amount xc.AmountBlockchain, contract xc.ContractAddress, txInput *TxInput) (xc.Tx, error) {
	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}

	ataFromStr, err := types.FindAssociatedTokenAddress(string(from), string(contract), txInput.TokenProgram)
	if err != nil {
		return nil, err
	}
	ataFrom := solana.MustPublicKeyFromBase58(ataFromStr)

	// instructions to:
	// - close the ATA (token.NewCloseAccountInstruction()) -- unwraps everything into from account
	instructions := []solana.Instruction{
		token.NewCloseAccountInstruction(ataFrom, accountFrom, accountFrom, nil).Build(),
	}

	return txBuilder.buildSolanaTx(from, from, instructions, txInput)
}
