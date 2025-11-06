package icp_test

import (
	"encoding/hex"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
	"github.com/stretchr/testify/require"
)

func TestTransferHash(t *testing.T) {
	expectedHash := "5e660afb6fdbc29176daf5f0c203280048f216a859ae8ccd6e3234bdcc04db5d"
	fromStr := "3b2bc5dc524d34f2dd1a7c16a90750ae2188e83c43b9505a342d5173a965f91e"
	from, err := hex.DecodeString(fromStr)
	require.NoError(t, err)
	toStr := "59fe1ba5f84295c1b737d0aa8b6211699f55896d62a0e141cd54b3ff5bea4e85"
	to, err := hex.DecodeString(toStr)
	require.NoError(t, err)

	transactionByteAddress := icp.Transaction[[]byte]{
		IcpMemo:       0,
		CreatedAtTime: nil,
		Operation: icp.Operation[[]byte]{
			Transfer: &icp.Transfer[[]byte]{
				From: from,
				To:   to,
				Amount: icp.Tokens{
					E8s: 1_014_395_588,
				},
				Fee: icp.Tokens{
					E8s: 10_000,
				},
				Spender: nil,
			},
		},
		Icrc1Memo: &[]byte{00, 00, 00, 00, 00, 00, 0x03, 0xfc},
	}

	h, err := transactionByteAddress.Hash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, h)

	// Test that `[string]` variant produce the same hash
	transactionStringAddress := icp.Transaction[string]{
		IcpMemo:       0,
		CreatedAtTime: nil,
		Operation: icp.Operation[string]{
			Transfer: &icp.Transfer[string]{
				From: fromStr,
				To:   toStr,
				Amount: icp.Tokens{
					E8s: 1_014_395_588,
				},
				Fee: icp.Tokens{
					E8s: 10_000,
				},
				Spender: nil,
			},
		},
		Icrc1Memo: &[]byte{00, 00, 00, 00, 00, 00, 0x03, 0xfc},
	}
	h, err = transactionStringAddress.Hash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, h)
}

func TestBurnHash(t *testing.T) {
	timestamp := icp.Timestamp{
		TimestampNanos: 1753126813775314320,
	}

	fromStr := "302b51fc2f5d5ce630fd1ca725c8ef01d2f01523ba531f4e581aa964ea1b2674"
	from, err := hex.DecodeString(fromStr)
	require.NoError(t, err)

	transaction := icp.Transaction[[]byte]{
		IcpMemo:       0,
		CreatedAtTime: &timestamp,
		Operation: icp.Operation[[]byte]{
			Burn: &icp.Burn[[]byte]{
				From: from,
				Amount: icp.Tokens{
					E8s: 59445250,
				},
				Spender: nil,
			},
		},
	}

	h, err := transaction.Hash()
	require.NoError(t, err)
	require.Equal(t, "1d1ce5312021042e67140ed2abcc81d2f5e2e073c65e66343876bf5a220ace88", h)

}

func TestMintHash(t *testing.T) {
	timestamp := icp.Timestamp{
		TimestampNanos: 1753130853654425885,
	}

	toStr := "8d5a08a6389cb2f75ca7fc74eb71c84ce15149b356fc1a573a5e8db1d50cfb0f"
	to, err := hex.DecodeString(toStr)
	require.NoError(t, err)

	transaction := icp.Transaction[[]byte]{
		IcpMemo:       1753130853,
		CreatedAtTime: &timestamp,
		Operation: icp.Operation[[]byte]{
			Mint: &icp.Mint[[]byte]{
				To: to,
				Amount: icp.Tokens{
					E8s: 941_545_518,
				},
			},
		},
	}

	h, err := transaction.Hash()
	require.NoError(t, err)
	require.Equal(t, "6def16f7d258e25aa061a3890b75f1bffa5b6d019efe2a713378e788d6b11973", h)
}
