package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx"
	"github.com/stretchr/testify/require"
)

func TestTxHash(t *testing.T) {
	pk := "bd08143ec55c47d3be603f8cf395025f8473d0e4d09a72eb83631fc1d745fb31"
	pkBytes, err := hex.DecodeString(pk)
	require.NoError(t, err)
	toStr := "723b365d1ca4bd14f4899cb3d6d028434e3219a9f4de98fa52553dfb6c363af4"
	to, err := hex.DecodeString(toStr)
	require.NoError(t, err)
	ts := icp.NewTimestamp(1_753_737_958_565_528_000)
	tx1 := tx.Tx{
		IcrcTransfer: nil,
		IcpTransfer: &icp.TransferArgs{
			To:             to,
			Fee:            icp.NewTokens(10_000),
			Memo:           0,
			FromSubaccount: nil,
			CreatedAtTime:  &ts,
			Amount:         icp.NewTokens(5_000_000),
		},
		Pubkey:   pkBytes,
		IsIcrcTx: false,
	}
	require.Equal(t, xc.TxHash("550322db518f06d67d5ce11df985e2651f553cebfd144eff5473a5d12e47aa1a"), tx1.Hash())
}
