package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/stretchr/testify/require"
)

func TestNativeTx(t *testing.T) {
	chain := xc.NewChainConfig(xc.TON).WithDecimals(9)
	builder, err := ton.NewTxBuilder(chain.Base())
	require.NoError(t, err)

	from := "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"
	to := "0QChotyiAtSPqs0BbPD851Mys9_LdMVM7N-atsFYvUMc48Jm"
	input := ton.NewTxInput()
	input.PublicKey, _ = hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	args := buildertest.MustNewTransferArgs(
		xc.Address(from),
		xc.Address(to),
		xc.NewAmountBlockchainFromUint64(10),
	)

	tx, err := builder.Transfer(args, input)
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
	chain := xc.NewChainConfig(xc.TON).WithDecimals(9)
	builder, err := ton.NewTxBuilder(chain.Base())
	require.NoError(t, err)

	from := "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5"
	to := "kQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9Wse1G"
	args := buildertest.MustNewTransferArgs(
		xc.Address(from),
		xc.Address(to),
		xc.NewAmountBlockchainFromUint64(10),
		buildertest.OptionContractAddress(xc.ContractAddress("kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di")),
	)

	input := ton.NewTxInput()
	input.PublicKey, _ = hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	input.TokenWallet = "EQA9NAsVDqwDkvX0KKv_3zwzvtaEaKf39gfEoT2AE-9WsVbM"
	input.JettonWalletCode, err = hex.DecodeString("b5ee9c7241021101000323000114ff00f4a413f4bcf2c80b0102016202030202cc0405001ba0f605da89a1f401f481f481a8610201d40607020120080900c30831c02497c138007434c0c05c6c2544d7c0fc03383e903e900c7e800c5c75c87e800c7e800c1cea6d0000b4c7e08403e29fa954882ea54c4d167c0278208405e3514654882ea58c511100fc02b80d60841657c1ef2ea4d67c02f817c12103fcbc2000113e910c1c2ebcb853600201200a0b0083d40106b90f6a2687d007d207d206a1802698fc1080bc6a28ca9105d41083deecbef09dd0958f97162e99f98fd001809d02811e428027d012c678b00e78b6664f6aa401f1503d33ffa00fa4021f001ed44d0fa00fa40fa40d4305136a1522ac705f2e2c128c2fff2e2c254344270542013541403c85004fa0258cf1601cf16ccc922c8cb0112f400f400cb00c920f9007074c8cb02ca07cbffc9d004fa40f40431fa0020d749c200f2e2c4778018c8cb055008cf1670fa0217cb6b13cc80c0201200d0e009e8210178d4519c8cb1f19cb3f5007fa0222cf165006cf1625fa025003cf16c95005cc2391729171e25008a813a08209c9c380a014bcf2e2c504c98040fb001023c85004fa0258cf1601cf16ccc9ed5402f73b51343e803e903e90350c0234cffe80145468017e903e9014d6f1c1551cdb5c150804d50500f214013e809633c58073c5b33248b232c044bd003d0032c0327e401c1d3232c0b281f2fff274140371c1472c7cb8b0c2be80146a2860822625a019ad822860822625a028062849e5c412440e0dd7c138c34975c2c0600f1000d73b51343e803e903e90350c01f4cffe803e900c145468549271c17cb8b049f0bffcb8b08160824c4b402805af3cb8b0e0841ef765f7b232c7c572cfd400fe8088b3c58073c5b25c60063232c14933c59c3e80b2dab33260103ec01004f214013e809633c58073c5b3327b552000705279a018a182107362d09cc8cb1f5230cb3f58fa025007cf165007cf16c9718010c8cb0524cf165006fa0215cb6a14ccc971fb0010241023007cc30023c200b08e218210d53276db708010c8cb055008cf165004fa0216cb6a12cb1f12cb3fc972fb0093356c21e203c85004fa0258cf1601cf16ccc9ed5495eaedd7")
	require.NoError(t, err)
	tx, err := builder.Transfer(args, input)
	require.NoError(t, err)

	// Should be an error if token wallet is not set
	input.TokenWallet = ""
	_, err = builder.Transfer(args, input)

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
	require.EqualValues(t, "57769ae8b851cc2c0d6ad68d6943a81bcfe37c44f582235cbb515f256f9dfe83", hash)

	// confirm bz - will need to change this when changing tx format.
	require.EqualValues(t,
		"b5ee9c720102050100019b0003e1880046fca233ff454051989f2b93946eacd0a5e3ba9a7d42ae09adef220caa0fbc94118000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005353462e000038400000000007001020300deff0020dd2082014c97ba218201339cbab19f71b0ed44d0d31fd31f31d70bffe304e0a4f2608308d71820d31fd31fd31ff82313bbf263ed44d0d31fd31fd3ffd15132baf2a15144baf2a204f901541055f910f2a3f8009320d74a96d307d402fb00e8d101a4c8cb1fcb1fcbffc9ed5400500000000029a9a317c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1016862001e9a058a875601c97afa1455ffef9e19df6b423453fbfb03e2509ec009f7ab58a05f5e1000000000000000000000000000010400a20f8a7ea5000000000000000010a8007a68162a1d580725ebe85157ffbe78677dad08d14fefec0f89427b0027dead630008df94467fe8a80a3313e572728dd59a14bc77534fa855c135bde4419541f79280",
		hex.EncodeToString(bz))

}
