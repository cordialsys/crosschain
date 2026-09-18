package tx_input_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestV1FeesAndLimits(t *testing.T) {
	input := tx_input.NewTxInput()
	input.TransactionVersion = "v1"
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
	input.TransactionVersion = "v2"
	_, err = input.MessageVersion()
	require.Error(t, err)
	var old tx_input.TxInput
	require.NoError(t, json.Unmarshal([]byte(`{}`), &old))
	version, err := old.MessageVersion()
	require.NoError(t, err)
	require.Equal(t, solana.MessageVersionLegacy, version)
}
