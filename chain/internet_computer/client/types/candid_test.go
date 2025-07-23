package types_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/stretchr/testify/require"
)

func TestTransferHash(t *testing.T) {
	timestamp := types.Timestamp{
		TimestampNanos: 1621901572293430780,
	}

	fromStr := "e7a879ea563d273c46dd28c1584eaa132fad6f3e316615b3eb657d067f3519b5"
	from, err := hex.DecodeString(fromStr)
	require.NoError(t, err)
	toStr := "207ec07185bedd0f2176ec2760057b8b7bc619a94d60e70fbc91af322a9f7e93"
	to, err := hex.DecodeString(toStr)
	require.NoError(t, err)

	transaction := types.Transaction{
		Memo:          5432845643782906771,
		CreatedAtTime: timestamp,
		Operation: &types.Operation{
			Transfer: &types.Transfer{
				From: from,
				To:   to,
				Amount: types.Tokens{
					E8s: 11541900000,
				},
				Fee: types.Tokens{
					E8s: 10_000,
				},
				Spender: nil,
			},
		},
	}

	h, err := transaction.Hash()
	require.NoError(t, err)
	require.Equal(t, "be31664ef154456aec5df2e4acc7f23a715ad8ea33ad9dbcbb7e6e90bc5a8b8f", h)
}

func TestBurnHash(t *testing.T) {
	timestamp := types.Timestamp{
		TimestampNanos: 1753126813775314320,
	}

	fromStr := "302b51fc2f5d5ce630fd1ca725c8ef01d2f01523ba531f4e581aa964ea1b2674"
	from, err := hex.DecodeString(fromStr)
	require.NoError(t, err)

	transaction := types.Transaction{
		Memo:          0,
		CreatedAtTime: timestamp,
		Operation: &types.Operation{
			Burn: &types.Burn{
				From: from,
				Amount: types.Tokens{
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
	timestamp := types.Timestamp{
		TimestampNanos: 1753130853654425885,
	}

	toStr := "8d5a08a6389cb2f75ca7fc74eb71c84ce15149b356fc1a573a5e8db1d50cfb0f"
	to, err := hex.DecodeString(toStr)
	require.NoError(t, err)

	transaction := types.Transaction{
		Memo:          1753130853,
		CreatedAtTime: timestamp,
		Operation: &types.Operation{
			Mint: &types.Mint{
				To: to,
				Amount: types.Tokens{
					E8s: 941_545_518,
				},
			},
		},
	}

	h, err := transaction.Hash()
	require.NoError(t, err)
	require.Equal(t, "6def16f7d258e25aa061a3890b75f1bffa5b6d019efe2a713378e788d6b11973", h)
}

func TestIcrc1AccountRoundtrip(t *testing.T) {
	test := "ztwhb-qiaaa-aaaaj-azw7a-cai-trfyica.e6fea3c3b4b55b57acd870361f93d4e30976d8dbfa5b6a8c7394dd0002"
	icrcAcc, err := types.DecodeICRC1Account(test)
	require.NoError(t, err)

	fmt.Printf("Icrc: %+v\n\n", icrcAcc)
	encoded := icrcAcc.Encode()

	fmt.Printf("Icrc: %s\n\n", encoded)
	require.Equal(t, test, encoded)
}

func TestIcrc1AccountRoundtripNoSubbacc(t *testing.T) {
	test := "ztwhb-qiaaa-aaaaj-azw7a-cai"
	icrcAcc, err := types.DecodeICRC1Account(test)
	require.NoError(t, err)

	fmt.Printf("Icrc: %+v\n\n", icrcAcc)
	encoded := icrcAcc.Encode()

	fmt.Printf("Icrc: %s\n\n", encoded)
	require.Equal(t, test, encoded)
}
