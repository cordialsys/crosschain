package tempo

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func TestTransferEncodesFeeContract(t *testing.T) {
	chain := xc.NewChainConfig("TEMPO").WithChainID("42429").Base()
	txBuilder, err := NewTxBuilder(chain)
	require.NoError(t, err)

	feeContract := xc.ContractAddress("0x20c0000000000000000000000000000000000001")
	args, err := xcbuilder.NewTransferArgs(
		chain,
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		xc.NewAmountBlockchainFromUint64(1),
		xcbuilder.OptionContractAddress("0x3333333333333333333333333333333333333333"),
		xcbuilder.OptionFeeContract(feeContract),
	)
	require.NoError(t, err)

	input := evminput.NewTxInput()
	input.ChainId = xc.NewAmountBlockchainFromUint64(42429)
	input.GasLimit = 100_000
	input.GasFeeCap = xc.NewAmountBlockchainFromUint64(2_000_000_000)
	input.GasTipCap = xc.NewAmountBlockchainFromUint64(1_000_000_000)

	tx, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	serialized, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t, tempoTransactionType, serialized[0])

	var fields []rlp.RawValue
	require.NoError(t, rlp.DecodeBytes(serialized[1:], &fields))
	require.Len(t, fields, 13)

	var encodedFeeContract []byte
	require.NoError(t, rlp.DecodeBytes(fields[10], &encodedFeeContract))
	require.Equal(t, common.HexToAddress(string(feeContract)).Bytes(), encodedFeeContract)
}

func TestTransferEncodesFeeContractFromPersistedInput(t *testing.T) {
	chain := xc.NewChainConfig("TEMPO").WithChainID("42431").Base()
	txBuilder, err := NewTxBuilder(chain)
	require.NoError(t, err)

	feeContract := xc.ContractAddress("0x20c0000000000000000000000000000000000001")
	args, err := xcbuilder.NewTransferArgs(
		chain,
		"0xfa201cdad10a60ae96e9dfcb60dbc7871e7355b8",
		"0x160daf34bbbab2fae98ed6d40d664061f54af387",
		xc.NewAmountBlockchainFromUint64(10_000_000),
		xcbuilder.OptionContractAddress("0x20c0000000000000000000000000000000000000"),
	)
	require.NoError(t, err)

	evmInput := evminput.NewTxInput()
	evmInput.ChainId = xc.NewAmountBlockchainFromUint64(42431)
	evmInput.Nonce = 11
	evmInput.GasLimit = 109_236
	evmInput.GasFeeCap = xc.NewAmountBlockchainFromUint64(68_445_424_287)
	evmInput.GasTipCap = xc.NewAmountBlockchainFromUint64(68_445_424_287)
	input := NewTxInputFromEVM(evmInput, feeContract)

	tx, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	serialized, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t, tempoTransactionType, serialized[0])

	var fields []rlp.RawValue
	require.NoError(t, rlp.DecodeBytes(serialized[1:], &fields))
	var encodedFeeContract []byte
	require.NoError(t, rlp.DecodeBytes(fields[10], &encodedFeeContract))
	require.Equal(t, common.HexToAddress(string(feeContract)).Bytes(), encodedFeeContract)
}

func TestNativeSponsorshipWithoutExplicitFeeContract(t *testing.T) {
	chain, _, input := feeTokenFixture(t)
	args, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient,
		xc.NewAmountBlockchainFromUint64(1_000_000),
		xcbuilder.OptionContractAddress(testTransferToken), xcbuilder.OptionFeePayer(testFeePayer, nil))
	require.NoError(t, err)
	input.NativeFeePayer, input.FeePayerAddress = true, testFeePayer
	built, err := buildTempoTransfer(chain, args, input)
	require.NoError(t, err)
	tx := built.(*Tx)
	require.Equal(t, testFeePayer, tx.feePayer)
	require.Equal(t, common.HexToAddress(string(testFeeToken)).Bytes(), tx.envelope.FeeToken)
	requests, err := tx.Sighashes()
	require.NoError(t, err)
	require.Equal(t, testSender, requests[0].Signer)
	require.Equal(t, "a856890727345b3cd0e2b589cd93aaacc6902c93afb0ea856fd72b29880181b3", fmt.Sprintf("%x", requests[0].Payload))
}

func TestTransferRejectsMismatchedInputs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*xcbuilder.TransferArgs, *TxInput)
		message string
	}{
		{"fee currency", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.FeeContract = testTransferToken }, "fee contract differs"},
		{"sender", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.FromAddress = testRecipient }, "sender differs"},
		{"missing sponsorship", func(a *xcbuilder.TransferArgs, _ *TxInput) { a.SetFeePayer(testFeePayer) }, "native sponsorship input"},
		{"unexpected sponsorship", func(_ *xcbuilder.TransferArgs, in *TxInput) {
			in.NativeFeePayer = true
			in.FeePayerAddress = testFeePayer
		}, "native sponsorship input"},
		{"payer mismatch", func(a *xcbuilder.TransferArgs, in *TxInput) {
			a.SetFeePayer(testFeePayer)
			in.NativeFeePayer = true
			in.FeePayerAddress = testRecipient
		}, "native sponsorship input"},
		{"payer nonce", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.FeePayerNonce = 1 }, "native sponsorship input"},
		{"smart account nonce", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.BasicSmartAccountNonce = 1 }, "native sponsorship input"},
		{"zero gas", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.GasLimit = 0 }, "gas limit"},
		{"gas caps", func(_ *xcbuilder.TransferArgs, in *TxInput) {
			in.GasTipCap = xc.NewAmountBlockchainFromUint64(30_000_000_000)
		}, "gas caps"},
		{"chain ID", func(_ *xcbuilder.TransferArgs, in *TxInput) { in.ChainId = xc.NewAmountBlockchainFromUint64(0) }, "chain ID"},
		{"payer address", func(a *xcbuilder.TransferArgs, _ *TxInput) { a.SetFeePayer("bad") }, "fee-payer address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain, args, input := feeTokenFixture(t)
			tc.mutate(&args, input)
			_, err := buildTempoTransfer(chain, args, input)
			require.ErrorContains(t, err, tc.message)
		})
	}
	chain, args, input := feeTokenFixture(t)
	b, err := NewTxBuilder(chain.Base())
	require.NoError(t, err)
	for _, in := range []xc.TxInput{nil, (*TxInput)(nil), (*evminput.TxInput)(nil)} {
		_, err := b.Transfer(args, in)
		require.Error(t, err)
	}
	args.SetFeePayer(testFeePayer)
	_, err = b.Transfer(args, &input.TxInput)
	require.ErrorContains(t, err, "requires a Tempo transaction input")
}

func TestSponsorshipInputMismatchDiagnostics(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	args.SetFeePayer(testFeePayer)
	// An input produced by the older EIP-7702 path has a payer and a payer
	// nonce, but no native sponsorship marker. Do not silently reinterpret it.
	input.FeePayerAddress = testFeePayer
	input.FeePayerNonce = 12
	input.BasicSmartAccountNonce = 3
	_, err := buildTempoTransfer(chain, args, input)
	require.ErrorContains(t, err, fmt.Sprintf("requested payer=%q", testFeePayer))
	require.ErrorContains(t, err, fmt.Sprintf("input payer=%q", testFeePayer))
	require.ErrorContains(t, err, "native_fee_payer=false (expected true)")
	require.ErrorContains(t, err, "fee_payer_nonce=12")
	require.ErrorContains(t, err, "basic_smart_account_nonce=3")
}
