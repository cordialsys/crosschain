package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/internet_computer/builder"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder, err := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	require.NotNil(t, builder)
	require.NoError(t, err)
}

func TestNewTransfer(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	from := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	to := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionPublicKey([]byte{1, 2, 3, 4}),
	)
	_, err := builder1.Transfer(args, input)
	require.NoError(t, err)
}

func TestNewTransferNoPublicKey(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	from := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	to := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
	)
	_, err := builder1.Transfer(args, input)
	require.ErrorContains(t, err, "missing public key")
}

func TESTNewTokenTransferInvalidContract(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	from := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	to := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("asdf")),
		buildertest.OptionPublicKey([]byte{1, 2, 3, 4}),
	)
	_, err := builder1.Transfer(args, input)
	require.NoError(t, err, "failed to decode canister")
}

func TestNewTokenTransfer(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	from := xc.Address("mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae")
	to := xc.Address("mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("n5wcd-faaaa-aaaar-qaaea-cai")),
		buildertest.OptionPublicKey([]byte{1, 2, 3, 4}),
	)
	_, err := builder1.Transfer(args, input)
	require.NoError(t, err, "missing public key")
}
