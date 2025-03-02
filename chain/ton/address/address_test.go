package address_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBuilder(t *testing.T) {
	builder, err := address.NewAddressBuilder(xc.NewChainConfig(""))
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestGetAddressFromPublicKey(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig(""))
	bytes, _ := hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	address, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("EQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSha2"), address)
}

func TestParseAddress(t *testing.T) {
	builder, _ := address.NewAddressBuilder(xc.NewChainConfig(""))
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
	chain := xc.NewChainConfig(xc.TON).WithNet("testnet")
	builder, _ := address.NewAddressBuilder(chain)
	bytes, _ := hex.DecodeString("c1172b7926116d2a396bd7d69b9880cc0657e8ba2db9f62b4c210c518321c8b1")
	derivedAddr, err := builder.GetAddressFromPublicKey(bytes)
	require.NoError(t, err)
	require.Equal(t, xc.Address("kQAjflEZ_6KgKMxPlcnKN1ZoUvHdTT6hVwTW95EGVQfeSq08"), derivedAddr)
}

// func TestParseAddressMetadata(t *testing.T) {
// 	addr1, err := tonutil.ParseAddr("EQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY0to")
// 	require.NoError(t, err)

// 	addr2, err := tonutil.ParseAddr("kQAiboDEv_qRrcEdrYdwbVLNOXBHwShFbtKGbQVJ2OKxY_Di")
// 	require.NoError(t, err)

// 	require.Equal(t, addr1, addr2)
// }
