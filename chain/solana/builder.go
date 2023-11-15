package solana

import (
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/sirupsen/logrus"
)

// TxBuilder for Solana
type TxBuilder struct {
	Asset xc.ITask
}

// Max number of token transfers we can fit in a solana transaction,
// when there's also a create ATA included.
const MaxTokenTransfers = 20

// NewTxBuilder creates a new Solana TxBuilder
func NewTxBuilder(asset xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: asset,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	switch asset := txBuilder.Asset.(type) {
	case *xc.TaskConfig:
		return txBuilder.NewTask(from, to, amount, input)
	case *xc.NativeAssetConfig:
		return txBuilder.NewNativeTransfer(from, to, amount, input)
	case *xc.TokenAssetConfig:
		return txBuilder.NewTokenTransfer(from, to, amount, input)
	default:
		// TODO this should return error
		contract, _ := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetNativeAsset().Asset,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("new transfer for unknown asset type")
		if contract != "" {
			return txBuilder.NewTokenTransfer(from, to, amount, input)
		} else {
			return txBuilder.NewNativeTransfer(from, to, amount, input)
		}
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}
	accountTo, err := solana.PublicKeyFromBase58(string(to))
	if err != nil {
		return nil, err
	}

	// txLog := map[string]string{
	// 	"type":      "system.Transfer",
	// 	"lamports":  amount.String(),
	// 	"funding":   accountFrom.String(),
	// 	"recipient": accountTo.String(),
	// }
	// log.Print(txLog)

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			system.NewTransferInstruction(
				amount.Uint64(),
				accountFrom,
				accountTo,
			).Build(),
		},
		input.(*TxInput).RecentBlockHash,
		solana.TransactionPayer(accountFrom),
	)
	if err != nil {
		return nil, err
	}
	return &Tx{
		SolTx: tx,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	asset := txBuilder.Asset
	txInput := input.(*TxInput)

	contract, ok := asset.GetContract()
	if !ok {
		return nil, errors.New("asset does not have a contract")
	}
	decimals, _ := asset.GetDecimals()

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

	ataFromStr, err := FindAssociatedTokenAddress(string(from), string(contract))
	if err != nil {
		return nil, err
	}
	ataFrom := solana.MustPublicKeyFromBase58(ataFromStr)
	if len(txInput.SourceTokenAccounts) > 0 {
		ataFrom = txInput.SourceTokenAccounts[0].Account
	}

	ataTo := accountTo
	if !txInput.ToIsATA {
		ataToStr, err := FindAssociatedTokenAddress(string(to), string(contract))
		if err != nil {
			return nil, err
		}
		ataTo = solana.MustPublicKeyFromBase58(ataToStr)
	}

	instructions := []solana.Instruction{}
	if txInput.ShouldCreateATA {
		instructions = append(instructions,
			ata.NewCreateInstruction(
				accountFrom,
				accountTo,
				accountContract,
			).Build(),
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
			if len(instructions) > MaxTokenTransfers {
				return nil, errors.New("cannot send total amount in single tx, try sending smaller amount")
			}
			if remainingBalanceToSend.Cmp(&zero) <= 0 {
				// we've spent enough from source accounts to meet target balance
				break
			}
		}
		if remainingBalanceToSend.Cmp(&zero) > 0 {
			return nil, errors.New("cannot send requested amount in single tx, try sending smaller amount")
		}
	}

	return txBuilder.buildSolanaTx(instructions, accountFrom, txInput)
}

func (txBuilder TxBuilder) buildSolanaTx(instructions []solana.Instruction, accountFrom solana.PublicKey, txInput *TxInput) (xc.Tx, error) {
	tx, err := solana.NewTransaction(
		instructions,
		txInput.RecentBlockHash,
		solana.TransactionPayer(accountFrom),
	)
	if err != nil {
		return nil, err
	}
	return &Tx{
		SolTx: tx,
	}, nil
}

func (txBuilder TxBuilder) NewTask(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*TxInput)
	task := txBuilder.Asset.(*xc.TaskConfig)
	switch task.Code {
	case "WrapTx":
		return txBuilder.BuildWrapTx(from, to, amount, txInput)
	case "UnwrapEverythingTx":
		return txBuilder.BuildUnwrapEverythingTx(from, to, amount, txInput)
	}
	return &Tx{}, fmt.Errorf("not implemented task: '%s'", txBuilder.Asset.ID())
}

func (txBuilder TxBuilder) BuildWrapTx(from xc.Address, to xc.Address, amount xc.AmountBlockchain, txInput *TxInput) (xc.Tx, error) {
	// use the dst asset
	task := txBuilder.Asset.(*xc.TaskConfig)
	asset := task.DstAsset

	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}

	contract, _ := asset.GetContract()
	accountContract, err := solana.PublicKeyFromBase58(string(contract))
	if err != nil {
		return nil, err
	}

	ataFromStr, err := FindAssociatedTokenAddress(string(from), string(contract))
	if err != nil {
		return nil, err
	}
	ataFrom := solana.MustPublicKeyFromBase58(ataFromStr)

	// instructions to:
	// - transfer to the ATA (system.NewTransferInstruction())
	// - create the ATA (associatedtokenaccount.NewCreateInstruction())
	instructions := []solana.Instruction{
		ata.NewCreateInstruction(
			accountFrom,
			accountFrom,
			accountContract,
		).Build(),
		system.NewTransferInstruction(
			amount.Uint64(),
			accountFrom,
			ataFrom,
		).Build(),
	}

	return txBuilder.buildSolanaTx(instructions, accountFrom, txInput)
}

func (txBuilder TxBuilder) BuildUnwrapEverythingTx(from xc.Address, to xc.Address, amount xc.AmountBlockchain, txInput *TxInput) (xc.Tx, error) {
	asset := txBuilder.Asset
	accountFrom, err := solana.PublicKeyFromBase58(string(from))
	if err != nil {
		return nil, err
	}

	contract, _ := asset.GetContract()
	ataFromStr, err := FindAssociatedTokenAddress(string(from), string(contract))
	if err != nil {
		return nil, err
	}
	ataFrom := solana.MustPublicKeyFromBase58(ataFromStr)

	// instructions to:
	// - close the ATA (token.NewCloseAccountInstruction()) -- unwraps everything into from account
	instructions := []solana.Instruction{
		token.NewCloseAccountInstruction(ataFrom, accountFrom, accountFrom, nil).Build(),
	}

	return txBuilder.buildSolanaTx(instructions, accountFrom, txInput)
}
