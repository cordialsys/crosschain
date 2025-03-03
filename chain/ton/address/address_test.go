package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig("").Base())
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("").Base())
	bytes, _ := hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"), address)
}

func TestParseAddress(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig("").Base())
	bytes, _ := hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	derivedAddr, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"), derivedAddr)

	addr, err := address.ParseAddress("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2", "testnet")
	require.NoError(t, err)
	require.Equal(t, "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2", addr.String())

	addr, err = address.ParseAddress("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2", "testnet")
	require.NoError(t, err)
	require.Equal(t, "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2", addr.String())

	addr, err = address.ParseAddress("0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5", "testnet")
	require.NoError(t, err)
	require.Equal(t, "0QAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSvD5", addr.String())

	// use alternative address format
	addr, err = address.ParseAddress("0:237E5119FFA2A028CC4F95C9CA37566852F1DD4D3EA15704D6F791065507DE4A", "mainnet")
	require.NoError(t, err)
	require.Equal(t, "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2", addr.String())
	// addr, err = address.ParseAddress("0:337339704B339026E9485A854FED6D412E4EA0508758F92FEC9730593DAE32E7", "testnet")
	// require.NoError(t, err)
	// require.Equal(t, "UQAzczlwSzOQJulIWoVP7W1BLk6gUIdY-S_slzBZPa4y5wn-", addr.String())
}

func TestParseTestnetAddress(t *testing.T) {
	chain := xc.NewChainConfig(xc.TON).WithNet("testnet").Base()
	builder, _ := address.NewAddressBuilder(chain)
	bytes, _ := hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	derivedAddr, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("kQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSq08"), derivedAddr)
}

func TestCalculateTokenWalletAddress(t *testing.T) {

	type testCase struct {
		name          string
		ownerAddr     xc.Address
		contractAddr  xc.ContractAddress
		walletCodeHex string
		walletAddr    xc.Address
		calc          func(ownerAddr xc.Address, contractAddr xc.ContractAddress, jettonWalletCode []byte) (xc.Address, error)
	}

	testCases := []testCase{
		{
			// This is the example they give in their docs
			// https://docs.ton.org/v3/guidelines/dapps/cookbook#how-to-calculate-users-jetton-wallet-address-offline
			name:         "v2 jetton wallet address",
			ownerAddr:    "UQANCZLrRHVnenvs31J26Y6vUcirln0-6zs_U18w93WaN2da",
			contractAddr: "EQCxE6mUtQJKFnGfaROTKOt1lZbDiiX1kCixRv7Nw2Id_sDs",
			// Can get wallet-code from explorer, e.g. https://tonviewer.com/EQDJXgXvdkTCG0N6-e6B1-fl1U1LnPHMYpqio3UEA9KAZ07j?section=method
			walletCodeHex: "b5ee9c72410101010023000842028f452d7a4dfd74066b682365177259ed05734435be76b5fd4bd5d8af2b7c3d68206bbf76",
			walletAddr:    xc.Address("EQAXgYVGR0rl2az6qPJcPlxFyiNKPCQckoI2ZXaGxLnWJDja"),
			calc:          address.CalculateTokenWalletAddressV2,
		},

		{
			// This is not documented but seems many contracts are using this
			// https://github.com/ton-blockchain/ton/blob/2a68c8610bf28b43b2019a479a70d0606c2a0aa1/crypto/func/auto-tests/legacy_tests/jetton-wallet/imports/jetton-utils.fc#L27
			name:          "legacy jetton wallet address",
			ownerAddr:     "EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2",
			contractAddr:  "EQDJXgXvdkTCG0N6-e6B1-fl1U1LnPHMYpqio3UEA9KAZ07j",
			walletCodeHex: "b5ee9c7241021101000323000114ff00f4a413f4bcf2c80b0102016202030202cc0405001ba0f605da89a1f401f481f481a8610201d40607020120080900c30831c02497c138007434c0c05c6c2544d7c0fc03383e903e900c7e800c5c75c87e800c7e800c1cea6d0000b4c7e08403e29fa954882ea54c4d167c0278208405e3514654882ea58c511100fc02b80d60841657c1ef2ea4d67c02f817c12103fcbc2000113e910c1c2ebcb853600201200a0b0083d40106b90f6a2687d007d207d206a1802698fc1080bc6a28ca9105d41083deecbef09dd0958f97162e99f98fd001809d02811e428027d012c678b00e78b6664f6aa401f1503d33ffa00fa4021f001ed44d0fa00fa40fa40d4305136a1522ac705f2e2c128c2fff2e2c254344270542013541403c85004fa0258cf1601cf16ccc922c8cb0112f400f400cb00c920f9007074c8cb02ca07cbffc9d004fa40f40431fa0020d749c200f2e2c4778018c8cb055008cf1670fa0217cb6b13cc80c0201200d0e009e8210178d4519c8cb1f19cb3f5007fa0222cf165006cf1625fa025003cf16c95005cc2391729171e25008a813a08209c9c380a014bcf2e2c504c98040fb001023c85004fa0258cf1601cf16ccc9ed5402f73b51343e803e903e90350c0234cffe80145468017e903e9014d6f1c1551cdb5c150804d50500f214013e809633c58073c5b33248b232c044bd003d0032c0327e401c1d3232c0b281f2fff274140371c1472c7cb8b0c2be80146a2860822625a019ad822860822625a028062849e5c412440e0dd7c138c34975c2c0600f1000d73b51343e803e903e90350c01f4cffe803e900c145468549271c17cb8b049f0bffcb8b08160824c4b402805af3cb8b0e0841ef765f7b232c7c572cfd400fe8088b3c58073c5b25c60063232c14933c59c3e80b2dab33260103ec01004f214013e809633c58073c5b3327b552000705279a018a182107362d09cc8cb1f5230cb3f58fa025007cf165007cf16c9718010c8cb0524cf165006fa0215cb6a14ccc971fb0010241023007cc30023c200b08e218210d53276db708010c8cb055008cf165004fa0216cb6a12cb1f12cb3fc972fb0093356c21e203c85004fa0258cf1601cf16ccc9ed5495eaedd7",
			walletAddr:    xc.Address("EQAiGa5nqV3o3B5gVhk3tSMxPaQ6Qy1QDHQmnwUKswrtdit9"),
			calc:          address.CalculateTokenWalletAddressLegacy,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			jettonWalletCode, err := hex.DecodeString(testCase.walletCodeHex)
			require.NoError(t, err)

			calculated, err := testCase.calc(
				testCase.ownerAddr,
				testCase.contractAddr,
				jettonWalletCode,
			)
			require.NoError(t, err)
			require.Equal(t, testCase.walletAddr, calculated)
		})
	}
}
