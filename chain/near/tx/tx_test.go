package tx_test

import (
	"encoding/hex"
	"math"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near/tx"
	"github.com/stretchr/testify/require"
)

// Build your transaction
func buildNativeTransaction() tx.Tx[tx.TransferAction] {
	// PublicKey
	pubKey := tx.PublicKey{
		KeyType: 0,
		Data: [32]byte{
			242, 31, 43, 12, 14, 227, 230, 61, 65, 10, 176, 49, 129, 185, 181, 215,
			154, 232, 219, 195, 85, 228, 110, 54, 119, 130, 185, 31, 12, 89, 12, 13,
		},
	}

	// BlockHash
	blockHash := [32]byte{
		41, 91, 198, 122, 73, 58, 81, 66, 48, 45, 229, 36, 209, 13, 239, 197,
		20, 41, 201, 128, 171, 215, 243, 224, 121, 34, 255, 111, 67, 69, 121, 178,
	}

	// Build transaction
	transaction := tx.Transaction[tx.TransferAction]{
		SignerID:   "f21f2b0c0ee3e63d410ab03181b9b5d79ae8dbc355e46e367782b91f0c590c0d",
		PublicKey:  pubKey,
		Nonce:      231253846000017,
		ReceiverID: "crosschainxc.testnet",
		BlockHash:  blockHash,
		Actions: []tx.TransferAction{
			{
				Type: tx.ActionTypeTransfer,
				Action: tx.Uint128{
					Lo: 2003764205206896640,
					Hi: 54210,
				},
			},
		},
	}

	return tx.Tx[tx.TransferAction]{
		Transaction: transaction,
		Signature:   tx.Signature{},
	}
}

func TestTxSighashes(t *testing.T) {

	nativeTx := buildNativeTransaction()
	expectedSighash := "9fabe1d1adb2e1d4326ca5c353d99afc45ebd1cc8e7e3f6a84717ddeecdd2e51"

	sighashes, err := nativeTx.Sighashes()
	require.NoError(t, err)
	require.NotNil(t, sighashes)
	require.Equal(t, expectedSighash, hex.EncodeToString(sighashes[0].Payload))
}

func TestTxHash(t *testing.T) {
	nativeTx := buildNativeTransaction()
	expectedHash := xc.TxHash("BkHqZqxvgyHn6cGeUMR6VkN5oYxhAHKCMPVGWXNsgDUk")
	hash := nativeTx.Hash()
	require.Equal(t, expectedHash, hash)

}

func TestUint128FromAmountBlockchain(t *testing.T) {
	tests := []struct {
		name        string
		amount      xc.AmountBlockchain
		expectedLo  uint64
		expectedHi  uint64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "zero value",
			amount:      xc.NewAmountBlockchainFromUint64(0),
			expectedLo:  0,
			expectedHi:  0,
			expectError: false,
		},
		{
			name:        "small positive value",
			amount:      xc.NewAmountBlockchainFromUint64(12345),
			expectedLo:  12345,
			expectedHi:  0,
			expectError: false,
		},
		{
			name:        "max uint64 value",
			amount:      xc.NewAmountBlockchainFromUint64(uint64(math.MaxUint64)),
			expectedLo:  ^uint64(0),
			expectedHi:  0,
			expectError: false,
		},
		{
			name: "overflow - 2^128",
			amount: func() xc.AmountBlockchain {
				v := xc.NewAmountBlockchainFromUint64(1)
				newV := v.Int().Lsh(v.Int(), 128)
				v = xc.NewAmountBlockchainFromStr(newV.String())
				return v
			}(),
			expectError: true,
			errorMsg:    "overflow uint128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tx.Uint128FromAmountBlockchain(tt.amount)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				require.Equal(t, tx.Uint128{}, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLo, result.Lo, "Lo mismatch")
				require.Equal(t, tt.expectedHi, result.Hi, "Hi mismatch")
			}
		})
	}
}
