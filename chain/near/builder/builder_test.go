package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/near/builder"
	nearerrors "github.com/cordialsys/crosschain/chain/near/errors"
	"github.com/cordialsys/crosschain/chain/near/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	chainCfg := xc.NewChainConfig(xc.NEAR).Base()
	builder, err := builder.NewTxBuilder(chainCfg)
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestNewNativeTransfer(t *testing.T) {
	chainCfg := xc.NewChainConfig(xc.NEAR).Base()
	builder, _ := builder.NewTxBuilder(chainCfg)
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}

	// Invalid input type
	var input xc.TxInput
	args := buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
	)
	_, err := builder.Transfer(args, input)
	require.ErrorContains(t, err, "invalid input type")

	// Missing public key
	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
	)
	_, err = builder.Transfer(args, input)
	require.ErrorContains(t, err, "public key is required for NEAR transactions")

	// Invalid public key length
	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionPublicKey([]byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		}),
	)
	_, err = builder.Transfer(args, input)
	require.ErrorContains(t, err, nearerrors.ErrInvalidPublicKeyLength.Error())

	// Valid args
	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionPublicKey([]byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		}),
	)
	_, err = builder.Transfer(args, input)
	require.NoError(t, err)
}

func TestNewTokenTransfer(t *testing.T) {
	chainCfg := xc.NewChainConfig(xc.NEAR).Base()
	builder1, _ := builder.NewTxBuilder(chainCfg)
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}

	var input xc.TxInput
	args := buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
	)
	_, err := builder1.Transfer(args, input)
	require.ErrorContains(t, err, "invalid input type")

	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
	)
	_, err = builder1.Transfer(args, input)
	require.ErrorContains(t, err, "public key is required for NEAR transactions")

	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
		buildertest.OptionPublicKey([]byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		}),
	)
	_, err = builder1.Transfer(args, input)
	require.ErrorContains(t, err, nearerrors.ErrInvalidPublicKeyLength.Error())

	// Valid args
	input = &TxInput{}
	args = buildertest.MustNewTransferArgs(
		chainCfg,
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
		buildertest.OptionPublicKey([]byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		}),
	)
	_, err = builder1.Transfer(args, input)
	require.NoError(t, err)
}
