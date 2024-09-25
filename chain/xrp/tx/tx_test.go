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

func TestTxSighashes(t *testing.T) {

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
