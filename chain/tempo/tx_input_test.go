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

func TestTempoNonceConflicts(t *testing.T) {
	native := func(from xc.Address, nonce uint64) *TxInput {
		in := NewTxInput()
		in.FromAddress, in.Nonce = from, nonce
		in.NativeFeePayer, in.FeePayerAddress = true, testFeePayer
		return in
	}
	ordinary := native(testRecipient, 7)
	ordinary.NativeFeePayer, ordinary.FeePayerAddress = false, ""
	legacy := &MultiTransferInput{TxInput: *native(testRecipient, 8)}
	legacy.NativeFeePayer = false
	legacy.FeePayerAddress, legacy.FeePayerNonce = testSender, 7
	unrelatedLegacy := &MultiTransferInput{TxInput: *native(testRecipient, 8)}
	unrelatedLegacy.NativeFeePayer = false
	unrelatedLegacy.FeePayerAddress, unrelatedLegacy.FeePayerNonce = testFeePayer, 0
	for _, tc := range []struct {
		name        string
		other       xc.TxInput
		independent bool
	}{
		{"shared sponsor", native(testRecipient, 7), true},
		{"same sender and nonce", native(testSender, 7), false},
		{"different nonce", native(testSender, 8), true},
		{"ordinary different sender same nonce", ordinary, true},
		{"EIP7702 payer consumes sender nonce", legacy, false},
		{"EIP7702 consumes native sponsor nonce", unrelatedLegacy, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := native(testSender, 7)
			require.Equal(t, tc.independent, a.IndependentOf(tc.other))
			require.Equal(t, tc.independent, tc.other.IndependentOf(a))
			require.Equal(t, !tc.independent, a.SafeFromDoubleSend(tc.other))
			require.Equal(t, !tc.independent, tc.other.SafeFromDoubleSend(a))
		})
	}
	a := native(testSender, 7)
	require.False(t, a.IndependentOf(nil))
	require.False(t, a.SafeFromDoubleSend(nil))
	require.False(t, a.SafeFromDoubleSend(NewTxInput()))
}
