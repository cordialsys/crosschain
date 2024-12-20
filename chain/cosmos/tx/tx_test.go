package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/stretchr/testify/require"
)

type Tx = tx.Tx

func TestTxHash(t *testing.T) {

	tx := Tx{ChainCfg: &xc.ChainConfig{}}
	hash := tx.Hash()
	require.Equal(t, "023bcbe3ae753509926a7b9aa62f631830cdab9b0a1ae1e9ddceceba85bd677d", string(hash))
}

func TestTxAddSignaturesErr(t *testing.T) {
	tx := Tx{ChainCfg: &xc.ChainConfig{}}
	err := tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "invalid signatures size")

	tx = Tx{ChainCfg: &xc.ChainConfig{}}
	err = tx.AddSignatures(xc.TxSignature{1, 2, 3})
	require.NoError(t, err)

	err = tx.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.NoError(t, err)

	bytes := make([]byte, 64)
	err = tx.AddSignatures(xc.TxSignature(bytes))
	require.NoError(t, err)

	err = tx.AddSignatures([]xc.TxSignature{bytes}...)
	require.NoError(t, err)
}

func TestTxSerialize(t *testing.T) {

	tx := Tx{ChainCfg: &xc.ChainConfig{}}
	serialized, err := tx.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, serialized)
}

func TestGetSighash(t *testing.T) {

	sighash := tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmos}, []byte{})
	// echo -n '' | openssl dgst -sha256
	require.Exactly(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hex.EncodeToString(sighash))

	sighash = tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmosEvmos}, []byte{})
	require.Exactly(t, "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", hex.EncodeToString(sighash))
}
