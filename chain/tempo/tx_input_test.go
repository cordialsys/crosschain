package tempo

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/stretchr/testify/require"
)

func TestGetFeeLimitRoundsTempoFeeUpToTokenPrecision(t *testing.T) {
	input := NewTxInputFromEVM(&evminput.TxInput{
		GasLimit:  378_468,
		GasFeeCap: xc.NewAmountBlockchainFromUint64(20_000_000_000),
	}, xc.ContractAddress("0x20c00000000000000000000014f22ca97301eb73"))

	feeLimit, contract := input.GetFeeLimit()

	require.Equal(t, "0x20c00000000000000000000014f22ca97301eb73", string(contract))
	require.Equal(t, "7570", feeLimit.String())
}

func TestGetFeeLimitLeavesExactTempoFeeUnchanged(t *testing.T) {
	input := NewTxInputFromEVM(&evminput.TxInput{
		GasLimit:  378_500,
		GasFeeCap: xc.NewAmountBlockchainFromUint64(20_000_000_000),
	}, xc.ContractAddress("0x20c00000000000000000000014f22ca97301eb73"))

	feeLimit, _ := input.GetFeeLimit()

	require.Equal(t, "7570", feeLimit.String())
}
