package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/require"
)

type Tx = tx.Tx

func TestTxHashErr(t *testing.T) {

	tx := Tx{}
	hash := tx.Hash()
	require.Equal(t, "", string(hash))
}

func TestTxSighashesErr(t *testing.T) {

	tx := Tx{}
	sighashes, err := tx.Sighashes()
	require.EqualError(t, err, "transaction not initialized")
	require.Nil(t, sighashes)
}

func TestTxAddSignaturesErr(t *testing.T) {
	cosmosCfg, err := types.MakeEncodingConfig(&xc.ChainConfig{ChainPrefix: "atom"})
	require.NoError(t, err)

	tx := Tx{}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{},
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{
			{},
		},
		// missing Builder
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{
			{
				PubKey:   &secp256k1.PubKey{},
				Data:     &signing.SingleSignatureData{SignMode: 0, Signature: nil},
				Sequence: 0,
			},
		},
		CosmosTxBuilder: cosmosCfg.TxConfig.NewTxBuilder(),
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "invalid signatures size")

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

	tx := Tx{}
	serialized, err := tx.Serialize()
	require.EqualError(t, err, "transaction not initialized")
	require.Equal(t, serialized, []byte{})
}

func TestGetSighash(t *testing.T) {

	sighash := tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmos}, []byte{})
	// echo -n '' | openssl dgst -sha256
	require.Exactly(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hex.EncodeToString(sighash))

	sighash = tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmosEvmos}, []byte{})
	require.Exactly(t, "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", hex.EncodeToString(sighash))
}
