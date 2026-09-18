package tx_input_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/solana-foundation/solana-go/v2"
	"github.com/stretchr/testify/require"
)

func TestV1FeesAndLimits(t *testing.T) {
	input := tx_input.NewTxInput()
	require.True(t, input.SupportsV1)
	input.ComputeUnitLimit = 1001
	input.PrioritizationFee = xc.NewAmountBlockchainFromUint64(1000)
	input.BaseFee = xc.NewAmountBlockchainFromUint64(5000)
	config, err := input.V1Config()
	require.NoError(t, err)
	require.Equal(t, uint64(2), *config.PriorityFee) // ceil(1.001 lamports)
	require.Equal(t, tx_input.MaxLoadedAccountsDataSizeLimit, *config.LoadedAccountsDataSizeLimit)
	fee, _ := input.GetFeeLimit()
	require.Equal(t, uint64(5002), fee.Uint64())
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	var restored tx_input.TxInput
	require.NoError(t, json.Unmarshal(encoded, &restored))
	restoredConfig, err := restored.V1Config()
	require.NoError(t, err)
	require.Equal(t, config, restoredConfig)
	input.ComputeUnitLimit = tx_input.MaxComputeUnitLimit + 1
	_, err = input.V1Config()
	require.Error(t, err)
	input.ComputeUnitLimit = 0
	input.PrioritizationFee = xc.NewAmountBlockchainFromStr("18446744073709551616000000")
	_, err = input.V1Config()
	require.Error(t, err)
	input.PrioritizationFee = xc.NewAmountBlockchainFromStr("-1")
	_, err = input.V1Config()
	require.Error(t, err)
	var old tx_input.TxInput
	require.NoError(t, json.Unmarshal([]byte(`{}`), &old))
	version := old.MessageVersion()
	require.Equal(t, solana.MessageVersionV0, version)
}

func TestSerializedInputVersions(t *testing.T) {
	for _, tc := range []struct {
		payload string
		version solana.MessageVersion
	}{
		{`{"type":"solana"}`, solana.MessageVersionV0},
		{`{"type":"solana","supports_v1":false}`, solana.MessageVersionV0},
		{`{"type":"solana","supports_v1":true}`, solana.MessageVersionV1},
	} {
		t.Run(tc.payload, func(t *testing.T) {
			input, err := drivers.UnmarshalTxInput([]byte(tc.payload))
			require.NoError(t, err)
			version := input.(*tx_input.TxInput).MessageVersion()
			require.Equal(t, tc.version, version)
		})
	}
}

func TestExplicitV1Config(t *testing.T) {
	input := tx_input.NewTxInput()
	config := solana.TransactionConfig{}.WithComputeUnitLimit(2000).WithPriorityFee(17)
	input.TransactionConfig = &config
	input.SignatureCount = 2
	input.BaseFee = xc.NewAmountBlockchainFromUint64(5000)
	// Explicit budgets take precedence over the derived transfer estimates.
	input.ComputeUnitLimit = 9999
	input.PrioritizationFee = xc.NewAmountBlockchainFromUint64(1_000_000)
	actual, err := input.V1Config()
	require.NoError(t, err)
	require.Equal(t, config, actual)
	require.Nil(t, actual.LoadedAccountsDataSizeLimit)
	require.Equal(t, uint32(2000), input.V1ComputeUnitLimit())
	fee, _ := input.GetFeeLimit()
	require.Equal(t, uint64(10017), fee.Uint64())
	wire, err := drivers.MarshalTxInput(input)
	require.NoError(t, err)
	restored, err := drivers.UnmarshalTxInput(wire)
	require.NoError(t, err)
	actual, err = restored.(*tx_input.TxInput).V1Config()
	require.NoError(t, err)
	require.Equal(t, config, actual)
	fee, _ = restored.GetFeeLimit()
	require.Equal(t, uint64(10017), fee.Uint64())

	// Omitted priority fees in explicit configs mean zero, not the RPC price.
	input.TransactionConfig = &solana.TransactionConfig{}
	priorityFee := input.V1PriorityFee()
	require.True(t, priorityFee.IsZero())
	require.Equal(t, tx_input.MaxComputeUnitLimit, input.V1ComputeUnitLimit())
	invalid := solana.TransactionConfig{}.WithComputeUnitLimit(tx_input.MaxComputeUnitLimit + 1)
	input.TransactionConfig = &invalid
	_, err = input.V1Config()
	require.Error(t, err)
}

func TestCallSharedConfig(t *testing.T) {
	var input tx_input.CallInput
	require.NoError(t, json.Unmarshal([]byte(`{"supports_v1":true,"signature_count":2}`), &input))
	config := solana.TransactionConfig{}.WithPriorityFee(17)
	input.TxInput.TransactionConfig = &config
	input.BaseFee = xc.NewAmountBlockchainFromUint64(2_000_000) // unused nonce rent
	require.Equal(t, uint8(2), input.TxInput.SignatureCount)
	fee, _ := input.GetFeeLimit()
	require.Equal(t, uint64(10017), fee.Uint64())
	encoded, err := json.Marshal(input)
	require.NoError(t, err)
	var restored tx_input.CallInput
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.Equal(t, input, restored)
	fee, _ = restored.GetFeeLimit()
	require.Equal(t, uint64(10017), fee.Uint64())
	legacy := &solana.Transaction{}
	legacy.Message.SetVersion(solana.MessageVersionLegacy)
	legacy.Message.Header.NumRequiredSignatures = 1
	restored.SetFeeConfig(legacy)
	require.Nil(t, restored.TransactionConfig)
	require.Equal(t, uint8(1), restored.SignatureCount)
	require.False(t, restored.SupportsV1)
}
