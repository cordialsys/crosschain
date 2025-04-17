package builder_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/solana/builder"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput
type Tx = tx.Tx

func TestNewTxBuilder(t *testing.T) {
	txBuilder, err := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	require.NoError(t, err)
	require.NotNil(t, txBuilder)
}

func TestNewNativeTransfer(t *testing.T) {

	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	to := xc.Address("BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11")
	amount := xc.NewAmountBlockchainFromUint64(1200000) // 1.2 SOL
	input := &tx_input.TxInput{}
	tx, err := builder.NewNativeTransfer(from, from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx := tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x2), solTx.Message.Instructions[0].ProgramIDIndex) // system tx
}

func TestNewNativeTransferErr(t *testing.T) {

	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())

	from := xc.Address("from") // fails on parsing from
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tx, err := builder.NewNativeTransfer(from, from, to, amount, input)
	require.Nil(t, tx)
	require.EqualError(t, err, "invalid length, expected 32, got 3")

	from = xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	// fails on parsing to
	tx, err = builder.NewNativeTransfer(from, from, to, amount, input)
	require.Nil(t, tx)
	require.EqualError(t, err, "invalid length, expected 32, got 2")
}

func TestNewTokenTransfer(t *testing.T) {

	contract := "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	args := buildertest.MustNewTransferArgs(
		xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb"),
		xc.Address("BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11"),
		xc.NewAmountBlockchainFromUint64(1200000),
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 6),
	)

	ataToStr, _ := types.FindAssociatedTokenAddress(string(args.GetTo()), string(contract), solana.TokenProgramID)
	ataTo := solana.MustPublicKeyFromBase58(ataToStr)

	// transfer to existing ATA
	input := &TxInput{}
	tx, err := builder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx := tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x4), solTx.Message.Instructions[0].ProgramIDIndex) // token tx
	require.Equal(t, ataTo, solTx.Message.AccountKeys[2])                       // destination

	// transfer to non-existing ATA: create
	input = &TxInput{ShouldCreateATA: true}
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 2, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x7), solTx.Message.Instructions[0].ProgramIDIndex)
	require.Equal(t, uint16(0x8), solTx.Message.Instructions[1].ProgramIDIndex)
	require.Equal(t, ataTo, solTx.Message.AccountKeys[1])

	// transfer to non-existing ATA & fee payer used: create using fee payer money
	feePayer := xc.Address("21yrAb33AQtNB43XWm2X9uKMXnTq8u9Wpzxzn8ZHEZBu")
	input = &TxInput{ShouldCreateATA: true}
	feePayerArgs := args
	feePayerArgs.SetFeePayer(feePayer)
	tx, err = builder.Transfer(feePayerArgs, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 2, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x8), solTx.Message.Instructions[0].ProgramIDIndex)
	require.Equal(t, uint16(0x9), solTx.Message.Instructions[1].ProgramIDIndex)
	// The create-ATA instruction should reference the fee payer as the account creator
	require.Equal(t, uint16(0), solTx.Message.Instructions[0].Accounts[0])
	require.Equal(t, ataTo.String(), solTx.Message.AccountKeys[2].String())
	// The fee payer should be the fee-payer address.
	require.EqualValues(t, feePayer, solTx.Message.AccountKeys[0].String())

	// transfer directly to ATA
	args = buildertest.MustNewTransferArgs(
		args.GetFrom(),
		xc.Address(ataToStr),
		args.GetAmount(),
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 6),
	)
	input = &TxInput{ToIsATA: true}
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x4), solTx.Message.Instructions[0].ProgramIDIndex) // token tx
	require.Equal(t, ataTo, solTx.Message.AccountKeys[2])                       // destination

	// invalid: direct to ATA, but ToIsATA: false
	args = buildertest.MustNewTransferArgs(
		args.GetFrom(),
		xc.Address(ataToStr),
		args.GetAmount(),
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 6),
	)
	input = &TxInput{ToIsATA: false}
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x4), solTx.Message.Instructions[0].ProgramIDIndex) // token tx
	require.NotEqual(t, ataTo, solTx.Message.AccountKeys[2])                    // destination
}

func validateTransferChecked(tx *solana.Transaction, instr *solana.CompiledInstruction) (*token.TransferChecked, error) {
	accs, _ := instr.ResolveInstructionAccounts(&tx.Message)
	inst, _ := token.DecodeInstruction(accs, instr.Data)
	transferChecked := *inst.Impl.(*token.TransferChecked)
	if len(transferChecked.Signers) > 0 {
		return &transferChecked, fmt.Errorf("should not send multisig transfers")
	}
	return &transferChecked, nil
}
func getTokenTransferAmount(tx *solana.Transaction, instr *solana.CompiledInstruction) uint64 {
	transferChecked, err := validateTransferChecked(tx, instr)
	if err != nil {
		panic(err)
	}
	return *transferChecked.Amount
}

func TestNewMultiTokenTransfer(t *testing.T) {

	contract := "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"
	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())

	from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	to := xc.Address("BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11")

	amountTooBig := xc.NewAmountBlockchainFromUint64(500)
	amountExact := xc.NewAmountBlockchainFromUint64(300)
	amountSmall1 := xc.NewAmountBlockchainFromUint64(100)
	amountSmall2 := xc.NewAmountBlockchainFromUint64(150)
	amountSmall3 := xc.NewAmountBlockchainFromUint64(200)

	ataToStr, err := types.FindAssociatedTokenAddress(string(to), string(contract), solana.TokenProgramID)
	require.NoError(t, err)
	ataTo := solana.MustPublicKeyFromBase58(ataToStr)

	// transfer to existing ATA
	input := &TxInput{
		SourceTokenAccounts: []*tx_input.TokenAccount{
			{
				Account: solana.PublicKey{},
				Balance: xc.NewAmountBlockchainFromUint64(100),
			},
			{
				Account: solana.PublicKey{},
				Balance: xc.NewAmountBlockchainFromUint64(100),
			},
			{
				Account: solana.PublicKey{},
				Balance: xc.NewAmountBlockchainFromUint64(100),
			},
		},
	}
	args := buildertest.MustNewTransferArgs(
		from,
		to,
		xc.NewAmountBlockchainFromUint64(0),
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 6),
	)
	args.SetAmount(amountTooBig)

	_, err = builder.Transfer(args, input)
	require.ErrorContains(t, err, "cannot send")

	args.SetAmount(amountExact)
	tx, err := builder.Transfer(args, input)
	require.NoError(t, err)
	solTx := tx.(*Tx).SolTx

	_, err = validateTransferChecked(solTx, &solTx.Message.Instructions[0])
	require.NoError(t, err)

	require.Equal(t, uint16(0x4), solTx.Message.Instructions[0].ProgramIDIndex) // token tx
	require.Equal(t, ataTo, solTx.Message.AccountKeys[2])                       // destination
	require.Equal(t, 3, len(solTx.Message.Instructions))
	// exactAmount should have 3 instructions, 100 amount each
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[0]))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[1]))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[2]))

	// amountSmall1 should just have 1 instruction (fits 1 token balance exact)
	args.SetAmount(amountSmall1)
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[0]))

	// amountSmall2 should just have 2 instruction (first 100, second 50)
	args.SetAmount(amountSmall2)
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 2, len(solTx.Message.Instructions))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[0]))
	require.EqualValues(t, 50, getTokenTransferAmount(solTx, &solTx.Message.Instructions[1]))

	// amountSmall3 should just have 3 instruction (first 100, second 100)
	args.SetAmount(amountSmall3)
	tx, err = builder.Transfer(args, input)
	require.NoError(t, err)
	solTx = tx.(*Tx).SolTx
	require.Equal(t, 2, len(solTx.Message.Instructions))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[0]))
	require.EqualValues(t, 100, getTokenTransferAmount(solTx, &solTx.Message.Instructions[1]))

}

func TestNewTokenTransferErr(t *testing.T) {
	// invalid from, to
	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	args := buildertest.MustNewTransferArgs(
		"from",
		"to",
		xc.AmountBlockchain{},
	)
	input := &TxInput{}
	_, err := txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "invalid length, expected 32, got 3")

	// invalid to
	args = buildertest.MustNewTransferArgs(
		"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
		"to",
		xc.AmountBlockchain{},
	)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "invalid length, expected 32, got 2")

	// invalid asset contract
	args = buildertest.MustNewTransferArgs(
		"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
		"BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11",
		xc.AmountBlockchain{},
		buildertest.OptionContractAddress("contract", 6),
	)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "invalid length, expected 32, got 6")

	// missing contract
	args = buildertest.MustNewTransferArgs(
		"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
		"BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11",
		xc.AmountBlockchain{},
		buildertest.OptionContractAddress("", 6),
	)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "asset does not have a contract")
}

func TestNewTransfer(t *testing.T) {

	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	to := xc.Address("BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11")
	amount := xc.NewAmountBlockchainFromUint64(1200000) // 1.2 SOL
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from,
		to,
		amount,
	)
	tx, err := builder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tx)
	solTx := tx.(*Tx).SolTx
	require.Equal(t, 0, len(solTx.Signatures))
	require.Equal(t, 1, len(solTx.Message.Instructions))
	require.Equal(t, uint16(0x2), solTx.Message.Instructions[0].ProgramIDIndex) // system tx
}

func Bytes32(i byte) []byte {
	bz := make([]byte, 32)
	bz[0] = i
	return bz
}

func TestNewTransferAsToken(t *testing.T) {

	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	to := xc.Address("BWbmXj5ckAaWCAtzMZ97qnJhBAKegoXtgNrv9BUpAB11")
	amount := xc.NewAmountBlockchainFromUint64(1200000) // 1.2 SOL

	type testcase struct {
		txInput               *TxInput
		expectedSourceAccount string
	}
	testcases := []testcase{
		{
			txInput: &TxInput{
				RecentBlockHash: solana.HashFromBytes(Bytes32(1)),
			},
			expectedSourceAccount: "DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA",
		},
		{
			txInput: &TxInput{
				RecentBlockHash: solana.HashFromBytes(Bytes32(1)),
				SourceTokenAccounts: []*tx_input.TokenAccount{
					{
						Account: solana.MustPublicKeyFromBase58("gCr8Xc43gEKntp7pjsBNq8qFHeUUdie2D7TrfbzPMJP"),
					},
				},
			},
			// should use new source account specified in txInput
			expectedSourceAccount: "gCr8Xc43gEKntp7pjsBNq8qFHeUUdie2D7TrfbzPMJP",
		},
	}
	for _, v := range testcases {
		args := buildertest.MustNewTransferArgs(
			from, to, amount,
			buildertest.OptionContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", 6),
		)

		tx, err := builder.Transfer(args, v.txInput)
		require.Nil(t, err)
		require.NotNil(t, tx)
		solTx := tx.(*Tx).SolTx
		require.Equal(t, 0, len(solTx.Signatures))
		require.Equal(t, 1, len(solTx.Message.Instructions))
		require.Equal(t, uint16(0x4), solTx.Message.Instructions[0].ProgramIDIndex) // token tx
		tokenTf, err := validateTransferChecked(solTx, &solTx.Message.Instructions[0])
		require.NoError(t, err)
		require.Equal(t, v.expectedSourceAccount, tokenTf.Accounts[0].PublicKey.String())
	}
}
