package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {

	type testcase struct {
		tx           tx.Tx
		expectedHash xc.TxHash
	}

	vectors := []testcase{
		{
			// Missing XRP Transaction
			tx:           tx.Tx{},
			expectedHash: "",
		},
		{
			// Missing LastLedgerSequence
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						XRPAmount: "10000000",
					},
					Destination:     "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:  0,
					Fee:             "10",
					Flags:           0,
					Sequence:        861823,
					SigningPubKey:   "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType: "Payment",
					TxnSignature:    "304402200b92d0b3a651877e89ec2904691637116e06ccacfeeafe47e901d4d6fa91b4c302207dcd149e8226a46b3c15baa6509fe423eb9ce27c0f136bbacd1988bd0c988c1b",
				},
			},
			expectedHash: "",
		},
		{
			// Missing TxnSignature
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						XRPAmount: "10000000",
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					Flags:              0,
					LastLedgerSequence: 1220981,
					Sequence:           861823,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
				},
			},
			expectedHash: "",
		},
		{
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						XRPAmount: "10000000",
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					Flags:              0,
					LastLedgerSequence: 1220981,
					Sequence:           861823,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
					TxnSignature:       "304402200b92d0b3a651877e89ec2904691637116e06ccacfeeafe47e901d4d6fa91b4c302207dcd149e8226a46b3c15baa6509fe423eb9ce27c0f136bbacd1988bd0c988c1b",
				},
			},
			expectedHash: "47f709b91e363cd6f316826a593b43b8aee80a596058361074ca73ff374cd8b6",
		},
		{
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						TokenAmount: &tx.Amount{
							Currency: "USD",
							Issuer:   "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
							Value:    "1.52",
						},
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					Flags:              0,
					LastLedgerSequence: 1220981,
					Sequence:           861823,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
					TxnSignature:       "304402200b92d0b3a651877e89ec2904691637116e06ccacfeeafe47e901d4d6fa91b4c302207dcd149e8226a46b3c15baa6509fe423eb9ce27c0f136bbacd1988bd0c988c1b",
				},
			},
			expectedHash: "187f1ac69f774346b220f92c9fb591c1bbb87a3877580d5caa16ea6bf7027595",
		},
	}

	for _, vector := range vectors {
		actualHash := vector.tx.Hash()
		require.Equal(t, vector.expectedHash, actualHash)
	}
}

func TestTxSighashes(t *testing.T) {

	type testcase struct {
		tx              tx.Tx
		expectedSigHash []xc.TxDataToSign
		err             string
	}

	vectors := []testcase{
		{
			// Missing XRP Transaction
			tx:              tx.Tx{},
			expectedSigHash: nil,
			err:             "missing XRP transaction",
		},
		{
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						XRPAmount: "10000000",
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					LastLedgerSequence: 1220981,
					Flags:              0,
					Sequence:           861823,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
					TxnSignature:       "304402200b92d0b3a651877e89ec2904691637116e06ccacfeeafe47e901d4d6fa91b4c302207dcd149e8226a46b3c15baa6509fe423eb9ce27c0f136bbacd1988bd0c988c1b",
				},
			},
			expectedSigHash: []xc.TxDataToSign{
				{
					0xee, 0xc0, 0x76, 0x5f, 0x60, 0xdc, 0x4, 0x2f,
					0x5e, 0x59, 0x4d, 0xa7, 0x61, 0xfe, 0xe2, 0xc2,
					0xbc, 0xff, 0xb4, 0x78, 0xb0, 0x14, 0x43, 0xa6,
					0xf, 0x33, 0x25, 0x3a, 0xc2, 0x77, 0x75, 0x61,
				},
			},
			err: "",
		},
		{
			tx: tx.Tx{
				XRPTx: &tx.XRPTransaction{
					Account: "r92tsEZEjK82wra6xaDvjZocKnR78VqpEM",
					Amount: tx.AmountBlockchain{
						TokenAmount: &tx.Amount{
							Currency: "USD",
							Issuer:   "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq",
							Value:    "1.52",
						},
					},
					Destination:        "rs2x5gvFupB22myz86BUu7m5F4YuizsFna",
					DestinationTag:     0,
					Fee:                "10",
					LastLedgerSequence: 1220981,
					Flags:              0,
					Sequence:           861823,
					SigningPubKey:      "0391e85c96feab1c71250308ef99375bb3fa9b846fc2c8b906976fa9ac4bed0857",
					TransactionType:    "Payment",
					TxnSignature:       "304402200b92d0b3a651877e89ec2904691637116e06ccacfeeafe47e901d4d6fa91b4c302207dcd149e8226a46b3c15baa6509fe423eb9ce27c0f136bbacd1988bd0c988c1b",
				},
			},
			expectedSigHash: []xc.TxDataToSign{
				{
					0xd, 0x2b, 0x87, 0x29, 0x94, 0x2d, 0x81, 0xd,
					0xd1, 0x17, 0xb9, 0xa2, 0xc6, 0x9, 0x37, 0x8f,
					0x72, 0x70, 0xe7, 0x8, 0x77, 0xbe, 0x2d, 0xe9,
					0x54, 0x91, 0xf1, 0xde, 0xf8, 0xe2, 0x2, 0x6d,
				},
			},
			err: "",
		},
	}

	for _, vector := range vectors {
		sigHash, err := vector.tx.Sighashes()
		require.Equal(t, vector.expectedSigHash, sigHash)
		if err != nil {
			require.Error(t, err)
		} else {
			require.Nil(t, err)
		}
	}
}

func TestTxAddSignature(t *testing.T) {
	tx1 := tx.Tx{
		TransactionSignature: []xc.TxSignature{},
	}
	err := tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction already signed")

	tx2 := tx.Tx{}
	err = tx2.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.EqualError(t, err, "signature must be 64 or 65 length serialized bytestring of r,s, and recovery byte")

	bytes := make([]byte, 64)
	tx3 := tx.Tx{
		XRPTx: &tx.XRPTransaction{},
	}
	err = tx3.AddSignatures([]xc.TxSignature{bytes}...)
	require.Nil(t, err)
}
