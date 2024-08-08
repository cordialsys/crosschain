package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/require"
)

func TestTxHashErr(t *testing.T) {
	tx := tx.Tx{}
	hash := tx.Hash()
	require.Equal(t, "", string(hash))
}

func TestTxSighashesErr(t *testing.T) {
	tx := tx.Tx{}
	sighashes, err := tx.Sighashes()
	require.EqualError(t, err, "transaction not initialized")
	require.Nil(t, sighashes)
}

func TestTxAddSignatureErr(t *testing.T) {
	tx1 := tx.Tx{}
	err := tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	err = tx1.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.EqualError(t, err, "transaction not initialized")

	bytes := make([]byte, 64)
	err = tx1.AddSignatures([]xc.TxSignature{bytes}...)
	require.EqualError(t, err, "transaction not initialized")

	tx1 = tx.Tx{SolTx: &solana.Transaction{}}
	err = tx1.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.EqualError(t, err, "invalid signature (3): 010203")

	bytes = make([]byte, 64)
	err = tx1.AddSignatures([]xc.TxSignature{bytes}...)
	require.NoError(t, err)
	require.Equal(t, 1, len(tx1.SolTx.Signatures))

	err = tx1.AddSignatures([]xc.TxSignature{}...)
	require.NoError(t, err)
	require.Equal(t, 0, len(tx1.SolTx.Signatures))
}

func TestTxSerialize(t *testing.T) {
	tx1 := tx.Tx{}
	serialized, err := tx1.Serialize()
	require.EqualError(t, err, "transaction not initialized")
	require.Equal(t, serialized, []byte{})

	tx1 = tx.Tx{SolTx: &solana.Transaction{}}
	serialized, err = tx1.Serialize()
	require.NoError(t, err)
	require.Equal(t, serialized, []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})
}
