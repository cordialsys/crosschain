package evm_legacy_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm_legacy"
	"github.com/test-go/testify/require"
)

func TestBuilderLegacyTransfer(t *testing.T) {
	// EVM legacy re-uses the EVM builder, but uses a different tx-input.
	// This ensures that the builder properly typecasts/converts to the evm input, avoiding any panic.
	b, _ := evm_legacy.NewTxBuilder(xc.NewChainConfig(""))

	from := "0x724435CC1B2821362c2CD425F2744Bd7347bf299"
	to := "0x3ad57b83B2E3dC5648F32e98e386935A9B10bb9F"
	amount := xc.NewAmountBlockchainFromUint64(100)
	input := evm_legacy.NewTxInput()

	fmt.Println("--- ", input.GetDriver())
	fmt.Printf("--- %T\n", input)

	input.GasTipCap = builder.GweiToWei(evm_legacy.DefaultMaxTipCapGwei - 1)
	trans, err := b.Transfer(buildertest.MustNewTransferArgs(xc.Address(from), xc.Address(to), amount), input)
	require.NoError(t, err)
	require.NotNil(t, trans)

}
