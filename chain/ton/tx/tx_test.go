package tx_test

import (
	"encoding/hex"
	"testing"

	"github.com/cordialsys/crosschain"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/stretchr/testify/require"
)

func TestNativeTx(t *testing.T) {

	builder, err := ton.NewTxBuilder(&crosschain.ChainConfig{Chain: xc.TON, Decimals: 9})
	require.NoError(t, err)

	from := "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"
	to := "0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm"
	input := ton.NewTxInput()
	input.PublicKey, _ = hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	tx, err := builder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(10), input)
	require.NoError(t, err)

	hashes, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, hashes, 1)

	sig := make([]byte, 64)
	err = tx.AddSignatures(xc.TxSignature(sig))
	require.NoError(t, err)

	bz, err := tx.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, bz)

	hash := tx.Hash()
	require.NotEmpty(t, hash)

	// valid hex as we choose hex to be canonical
	_, err = hex.DecodeString(string(hash))
	require.NoError(t, err)
	require.EqualValues(t, "9ecf6fbb52b3d87d56f04c281e0e6a63f0f1e6938d5fe7350ec3071eaf0ddf1e", hash)

	// confirm bz - will need to change this when changing tx format.
	require.EqualValues(t,
		"b5ee9c72010204010001440003e1880046fca233ff454051989f2b93946eacd0a5e3ba9a7d42ae09adef220caa0fbc94118000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005353462e000038400000000007001020300deff0020dd2082014c97ba218201339cbab19f71b0ed44d0d31fd31f31d70bffe304e0a4f2608308d71820d31fd31fd31ff82313bbf263ed44d0d31fd31fd3ffd15132baf2a15144baf2a204f901541055f910f2a3f8009320d74a96d307d402fb00e8d101a4c8cb1fcb1fcbffc9ed5400500000000029a9a317c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b10062420050d16e51016a47d56680b6787e73a99959efe5ba62a6766fcd5b60ac5ea18e71885000000000000000000000000000",
		hex.EncodeToString(bz))

}

func TestTokenTx(t *testing.T) {
	chain := &crosschain.ChainConfig{Chain: xc.TON, Decimals: 9}
	builder, err := ton.NewTxBuilder(&crosschain.TokenAssetConfig{Chain: xc.TON, Decimals: 9, Contract: "kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di", ChainConfig: chain})
	require.NoError(t, err)

	from := "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"
	to := "0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm"
	input := ton.NewTxInput()
	input.PublicKey, _ = hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	input.TokenWallet = "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"
	tx, err := builder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(10), input)
	require.NoError(t, err)

	// Should be an error if token wallet is not set
	input.TokenWallet = ""
	_, err = builder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(10), input)
	require.Error(t, err)

	hashes, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, hashes, 1)

	sig := make([]byte, 64)
	err = tx.AddSignatures(xc.TxSignature(sig))
	require.NoError(t, err)

	bz, err := tx.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, bz)

	hash := tx.Hash()
	require.NotEmpty(t, hash)

	// valid hex as we choose hex to be canonical
	_, err = hex.DecodeString(string(hash))
	require.NoError(t, err)
	require.EqualValues(t, "69104f5f6acc7e518f8d9d0404e6891b3286a5f78c2d3b7de9665d5d48e94399", hash)

	// confirm bz - will need to change this when changing tx format.
	require.EqualValues(t,
		"b5ee9c720102050100019b0003e1880046fca233ff454051989f2b93946eacd0a5e3ba9a7d42ae09adef220caa0fbc94118000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005353462e000038400000000007001020300deff0020dd2082014c97ba218201339cbab19f71b0ed44d0d31fd31f31d70bffe304e0a4f2608308d71820d31fd31fd31ff82313bbf263ed44d0d31fd31fd3ffd15132baf2a15144baf2a204f901541055f910f2a3f8009320d74a96d307d402fb00e8d101a4c8cb1fcb1fcbffc9ed5400500000000029a9a317c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b10168620011bf288cffd150146627cae4e51bab342978eea69f50ab826b7bc8832a83ef25205f5e1000000000000000000000000000010400a20f8a7ea5000000000000000010a8014345b94405a91f559a02d9e1f9cea66567bf96e98a99d9bf356d82b17a8639c70008df94467fe8a80a3313e572728dd59a14bc77534fa855c135bde4419541f79280",
		hex.EncodeToString(bz))

}
