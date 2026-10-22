package bcs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// This vector was decoded and accepted by the real Sui devnet fullnode, and
// executed successfully. It locks both new enum discriminants and field order.
func TestDevnetAddressBalanceTransaction(t *testing.T) {
	file, err := os.ReadFile("testdata/address_balance_transaction.json")
	require.NoError(t, err)
	var fixture struct {
		Bcs   []byte `json:"bcs"`
		Epoch uint64 `json:"epoch,string"`
		Nonce uint32 `json:"nonce"`
	}
	require.NoError(t, json.Unmarshal(file, &fixture))
	tx, err := BcsDeserializeTransactionData(fixture.Bcs)
	require.NoError(t, err)
	v1 := tx.(*TransactionData__V1).Value
	require.Empty(t, v1.GasData.Payment)
	expiry := v1.Expiration.(*TransactionExpiration__ValidDuring)
	require.Equal(t, fixture.Epoch, *expiry.MinEpoch)
	require.Equal(t, fixture.Epoch, *expiry.MaxEpoch)
	require.Equal(t, fixture.Nonce, expiry.Nonce)
	require.Len(t, expiry.Chain, 32)
	ptb := v1.Kind.(*TransactionKind__ProgrammableTransaction).Value
	require.Len(t, ptb.Inputs, 2)
	require.Len(t, ptb.Commands, 2)
	withdrawal := ptb.Inputs[0].(*CallArg__FundsWithdrawal)
	require.Equal(t, uint64(10_000_000), withdrawal.Amount)
	require.False(t, withdrawal.Sponsor)
	tag := withdrawal.CoinType.(*TypeTag__Struct).Value
	require.Equal(t, AccountAddress{31: 2}, tag.Address)
	require.Equal(t, Identifier("sui"), tag.Module)
	require.Equal(t, Identifier("SUI"), tag.Name)
	wire, err := tx.BcsSerialize()
	require.NoError(t, err)
	require.Equal(t, fixture.Bcs, wire)
	for _, wire := range [][]byte{{2}, {2, 1}, {2, 0, 1}} {
		_, err := BcsDeserializeCallArg(wire)
		require.Error(t, err)
	}
	_, err = BcsDeserializeTransactionExpiration([]byte{2, 1})
	require.Error(t, err)
}
