package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {

	tx1 := tx.Tx{}
	hash := tx1.Hash()
	require.Equal(t, "", string(hash))
}

func TestTxSighashesErr(t *testing.T) {

	tx1 := tx.Tx{}
	sighashes1, err1 := tx1.Sighashes()
	require.EqualError(t, err1, "missing XRP transaction")
	require.Nil(t, sighashes1)

	tx2 := tx.Tx{
		XRPTx: &tx.XRPTransaction{},
	}
	sighashes2, err2 := tx2.Sighashes()
	require.EqualError(t, err2, "missing serialised XRP transaction")
	require.Nil(t, sighashes2)

	hexString := "5354580012000022000000002400070807201B000C44F96140000000014FB18068400000000000000A7321039543A0D3004CDA0904A09FB3710251C652D69EA338589279BC849D47A7B019A18114E2AFBD269D7DA5E2B9931CCBD62FAB5118A366188314BA4BEC4015A7CA2D99BE3319F488E0CA983D5506"
	encodeForSigning, _ := hex.DecodeString(hexString)
	tx3 := tx.Tx{
		XRPTx:            &tx.XRPTransaction{},
		EncodeForSigning: encodeForSigning,
	}
	sighashes3, err3 := tx3.Sighashes()
	require.Nil(t, err3)
	require.NotNil(t, sighashes3)
}

func TestTxSighashes(t *testing.T) {

	type testcase struct {
		XRPTx               *tx.XRPTransaction
		EncodeForSigningHex string

		SigHash []xc.TxDataToSign
	}
	//startTime := int64((100 * time.Hour).Seconds())
	vectors := []testcase{
		{
			XRPTx: &tx.XRPTransaction{
				Account:            "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH",
				Amount:             tx.AmountBlockchain{StringValue: "10"},
				Destination:        "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
				Fee:                "10",
				Sequence:           460817,
				Flags:              0,
				LastLedgerSequence: 1011094,
				SigningPubKey:      "039543A0D3004CDA0904A09FB3710251C652D69EA338589279BC849D47A7B019A1",
				TransactionType:    "Payment",
			},
			EncodeForSigningHex: "5354580012000022000000002400070811201B000F6D9661400000000098968068400000000000000A7321039543A0D3004CDA0904A09FB3710251C652D69EA338589279BC849D47A7B019A18114E2AFBD269D7DA5E2B9931CCBD62FAB5118A366188314BA4BEC4015A7CA2D99BE3319F488E0CA983D5506",
			SigHash: []xc.TxDataToSign{
				{
					0xa7, 0xb9, 0x3e, 0x26, 0x85, 0xed, 0x8d, 0x98,
					0xb4, 0x31, 0x8e, 0x7e, 0xd8, 0xb9, 0xa9, 0xae,
					0xb0, 0xa9, 0x3f, 0x7e, 0x37, 0x1c, 0x85, 0xca,
					0x94, 0xc9, 0x5c, 0xb1, 0xa4, 0x47, 0xb7, 0xe4,
				},
			},
		},
		{
			XRPTx: &tx.XRPTransaction{
				Account:            "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH",
				Amount:             tx.AmountBlockchain{AmountValue: &tx.Amount{Currency: "USD", Issuer: "rhub8VRN55s94qWKDv6jmDy1pUykJzF3wq", Value: "0.02"}},
				Destination:        "rHzsdt8NDw1R4YTDHvJgW8zt15AEKSgf1S",
				Fee:                "10",
				Sequence:           460817,
				Flags:              0,
				LastLedgerSequence: 1011094,
				SigningPubKey:      "039543A0D3004CDA0904A09FB3710251C652D69EA338589279BC849D47A7B019A1",
				TransactionType:    "Payment",
			},
			EncodeForSigningHex: "5354580012000022000000002400070811201B000F6D9661D4071AFD498D000000000000000000000000000055534400000000002ADB0B3959D60A6E6991F729E1918B716392523068400000000000000A7321039543A0D3004CDA0904A09FB3710251C652D69EA338589279BC849D47A7B019A18114E2AFBD269D7DA5E2B9931CCBD62FAB5118A366188314BA4BEC4015A7CA2D99BE3319F488E0CA983D5506",
			SigHash: []xc.TxDataToSign{
				{
					0xc0, 0x21, 0x67, 0xc1, 0x35, 0xa7, 0x04, 0xcd,
					0xb4, 0x00, 0x8d, 0xeb, 0x5b, 0x7e, 0xcc, 0x5c,
					0x06, 0x00, 0xe5, 0x7f, 0x8e, 0xda, 0x06, 0x16,
					0x7e, 0xfc, 0x8e, 0x74, 0x62, 0x41, 0x7c, 0xc3,
				},
			},
		},
	}
	for _, v := range vectors {

		encodeForSigningBytes, _ := hex.DecodeString(v.EncodeForSigningHex)
		tx3 := tx.Tx{
			XRPTx:            &tx.XRPTransaction{},
			EncodeForSigning: encodeForSigningBytes,
		}
		sighashes, err := tx3.Sighashes()
		require.Nil(t, err)
		require.NotNil(t, sighashes)
		require.Equal(t, sighashes[0], v.SigHash[0])
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
