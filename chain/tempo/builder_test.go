package tempo

import (
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
