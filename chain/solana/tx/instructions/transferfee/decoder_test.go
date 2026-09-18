package transferfee_test

import (
	"encoding/hex"
	"testing"

	"github.com/cordialsys/crosschain/chain/solana/tx/instructions/transferfee"
	"github.com/solana-foundation/solana-go/v2"
	"github.com/stretchr/testify/require"
)

func TestDecodeTransferCheckedWithFee(t *testing.T) {
	// Token-2022 transfer of 1000000 units with 6 decimals and a 1000-unit fee.
	data, err := hex.DecodeString("1a0140420f000000000006e803000000000000")
	require.NoError(t, err)
	accounts := []*solana.AccountMeta{
		solana.Meta(solana.PublicKey{1}).WRITE(), solana.Meta(solana.PublicKey{2}),
		solana.Meta(solana.PublicKey{3}).WRITE(), solana.Meta(solana.PublicKey{4}).SIGNER(),
	}
	decoded, err := transferfee.DecodeInstruction(accounts, data)
	require.NoError(t, err)
	transfer := decoded.Impl.(*transferfee.TransferCheckedWithFee)
	require.Equal(t, uint64(1000000), *transfer.Amount)
	require.Equal(t, uint8(6), *transfer.Decimals)
	require.Equal(t, uint64(1000), *transfer.Fee)
	require.Equal(t, accounts[0], transfer.GetSourceAccount())
	require.Equal(t, accounts[1], transfer.GetMintAccount())
	require.Equal(t, accounts[2], transfer.GetDestinationAccount())
	for length := 0; length < len(data); length++ {
		_, err = transferfee.DecodeInstruction(accounts, data[:length])
		require.Error(t, err)
	}
	_, err = transferfee.DecodeInstruction(accounts[:3], data)
	require.Error(t, err)
	data[1] = 2
	_, err = transferfee.DecodeInstruction(accounts, data)
	require.Error(t, err)
}
