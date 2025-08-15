package builder_test

import (
	"encoding/hex"
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
	chainCfg := xc.NewChainConfig(xc.ICP).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
		buildertest.OptionPublicKey([]byte{1, 2, 3, 4}),
	)
	txI, err := builder1.Transfer(args, input)
	require.NoError(t, err)

	err = txI.SetSignatures(&xc.SignatureResponse{
		Signature: make([]byte, 64),
	})
	require.NoError(t, err)

	bz, err := txI.Serialize()
	require.NoError(t, err)

	// Pin result to guard against non-determinism or unexpected changes.
	require.Equal(t,
		"a367636f6e74656e74a76361726758824449444c066d7b6c01e0a9b302786e006c01d6f68e8001786e036c06fbca0100c6fcb60201ba89e5c20478a2de94eb060282f3f3910c04d8a38ca80d010105206c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc889900000000000000000000000000000000000100000000000000000000000000000000656e6f6e63654a000000000000000000006673656e646572581dc0e2fee0ef1f2663f31387eab530ba3ecfbcea1913c6ac0cab9cc1dd026b63616e69737465725f69644a000000000000000201016b6d6574686f645f6e616d65687472616e736665726c726571756573745f747970656463616c6c6e696e67726573735f6578706972791b00000045d964b8006d73656e6465725f7075626b657950300e300506032b6570030500010203046a73656e6465725f736967584000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(bz))
}

func TestNewTransferNoPublicKey(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("ICP").Base())
	from := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	to := xc.Address("6c5066261553064a8d4fa8f30fa9d587d9887bce69601cdb5b6cac8780fc8899")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	chainCfg := xc.NewChainConfig(xc.ICP).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
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
	chainCfg := xc.NewChainConfig(xc.ICP).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
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
	chainCfg := xc.NewChainConfig(xc.ICP).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("n5wcd-faaaa-aaaar-qaaea-cai")),
		buildertest.OptionPublicKey([]byte{1, 2, 3, 4}),
	)
	txI, err := builder1.Transfer(args, input)
	require.NoError(t, err, "missing public key")

	err = txI.SetSignatures(&xc.SignatureResponse{
		Signature: make([]byte, 64),
	})
	require.NoError(t, err)

	bz, err := txI.Serialize()
	require.NoError(t, err)

	// Pin result to guard against non-determinism or unexpected changes.
	require.Equal(t,
		"a367636f6e74656e74a763617267586b4449444c056d7b6e006c02b3b0dac30368ad86ca8305016e786c06fbca0102c6fcb6027dba89e5c20401a2de94eb060182f3f3910c03d8a38ca80d7d0104011db9264e4ed0eb963400b9b36bca08a5c52dc7d9903a409af7c84c6a32020000000001000000000000000000656e6f6e63654a000000000000000000006673656e646572581dc0e2fee0ef1f2663f31387eab530ba3ecfbcea1913c6ac0cab9cc1dd026b63616e69737465725f69644a000000000230000801016b6d6574686f645f6e616d656e69637263315f7472616e736665726c726571756573745f747970656463616c6c6e696e67726573735f6578706972791b00000045d964b8006d73656e6465725f7075626b657950300e300506032b6570030500010203046a73656e6465725f736967584000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(bz))
}
