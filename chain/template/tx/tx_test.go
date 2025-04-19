package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/tx"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {

	tx1 := tx.Tx{}
	require.Equal(t, xc.TxHash("not implemented"), tx1.Hash())
}

func TestTxSighashes(t *testing.T) {

	tx1 := tx.Tx{}
	sighashes, err := tx1.Sighashes()
	require.NotNil(t, sighashes)
	require.EqualError(t, err, "not implemented")
}

func TestTxAddSignature(t *testing.T) {

	tx1 := tx.Tx{}
	err := tx1.AddSignatures([]*xc.SignatureResponse{}...)
	require.EqualError(t, err, "not implemented")
}
