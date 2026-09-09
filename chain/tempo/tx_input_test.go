package tempo

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/stretchr/testify/require"
)

func TestTxInputFeeContractJSON(t *testing.T) {
	const contract = xc.ContractAddress("0x20c00000000000000000000014f22ca97301eb73")
	input := NewTxInputFromEVM(&evminput.TxInput{}, contract)

	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	var serialized map[string]any
	require.NoError(t, json.Unmarshal(encoded, &serialized))
	require.Equal(t, string(contract), serialized["fee_contract"])

	var decoded TxInput
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, contract, decoded.FeeContract)
}

func TestGetFeeLimitSupportsFeeContract(t *testing.T) {
	const contract = xc.ContractAddress("0x20c00000000000000000000014f22ca97301eb73")
	input := NewTxInput()
	input.FeeContract = contract

	_, feeContract := input.GetFeeLimit()
	require.Equal(t, contract, feeContract)
}

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
