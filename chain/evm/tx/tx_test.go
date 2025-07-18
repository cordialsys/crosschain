package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/stretchr/testify/require"
)

func TestTxHashEmpty(t *testing.T) {
	tx := tx.Tx{}
	require.Equal(t, xc.TxHash(""), tx.Hash())
}

func TestTxSighashesEmpty(t *testing.T) {
	tx := tx.Tx{}
	_, err := tx.Sighashes()
	require.EqualError(t, err, "transaction not initialized")
}

func TestTxAddSignatureEmpty(t *testing.T) {
	tx := tx.Tx{}
	err := tx.SetSignatures([]*xc.SignatureResponse{}...)
	require.EqualError(t, err, "transaction not initialized")
}
