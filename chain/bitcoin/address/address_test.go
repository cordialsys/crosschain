package address_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/stretchr/testify/require"
)

var UXTO_ASSETS []xc.NativeAsset = []xc.NativeAsset{
	xc.BTC,
	xc.DOGE,
	xc.LTC,
}

// Address

func TestNewAddressBuilder(t *testing.T) {
	require := require.New(t)
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset)
		builder, err := address.NewAddressBuilder(chain.Base())
		require.NotNil(builder)
		require.NoError(err)
	}
}

func TestNewAddressBuilderInvalidAlgorithm(t *testing.T) {
	require := require.New(t)
	chain := xc.NewChainConfig(xc.BTC)
	_, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(xc.Ed255))
	require.ErrorContains(err, "ed255")
}

func TestNewAddressBuilderValidAlgorithms(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		name                string
		asset               xc.NativeAsset
		algorithm           xc.SignatureType
		expectedAlgorithm   xc.SignatureType
		expectedAddressType xc.AddressFormat
	}{
		{
			name:                "taproot-address",
			asset:               xc.BTC,
			algorithm:           xc.Schnorr,
			expectedAlgorithm:   xc.Schnorr,
			expectedAddressType: address.AddressTypeTaproot,
		},
		{
			name:                "legacy-address",
			asset:               xc.DOGE,
			algorithm:           xc.K256Sha256,
			expectedAlgorithm:   xc.K256Sha256,
			expectedAddressType: address.AddressTypeLegacy,
		},
		{
			name:                "segwit-explicit-algo-address",
			asset:               xc.BTC,
			algorithm:           xc.K256Sha256,
			expectedAlgorithm:   xc.K256Sha256,
			expectedAddressType: address.AddressTypeSegWit,
		},
		{
			name:                "segwit-missing-algo-address",
			asset:               xc.BTC,
			algorithm:           "",
			expectedAlgorithm:   xc.K256Sha256,
			expectedAddressType: address.AddressTypeSegWit,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chain := xc.NewChainConfig(tc.asset, tc.asset.Driver())

			// 1. Use using the algorithm as input
			builder, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(tc.algorithm))
			require.NoError(err)

			addressType := builder.(*address.AddressBuilder).GetAddressType()
			require.Equal(tc.expectedAddressType, addressType)

			alg := builder.(xc.AddressBuilderWithFormats).GetSignatureAlgorithm()
			require.Equal(tc.expectedAlgorithm, alg)

			// 2. Use using just the format as input
			builder, err = address.NewAddressBuilder(chain.Base(), xcaddress.OptionFormat(tc.expectedAddressType))
			require.NoError(err)

			addressType = builder.(*address.AddressBuilder).GetAddressType()
			require.Equal(tc.expectedAddressType, addressType)

			alg = builder.(xc.AddressBuilderWithFormats).GetSignatureAlgorithm()
			require.Equal(tc.expectedAlgorithm, alg)
		})
	}
}

func TestGetAddressFromPublicKey(t *testing.T) {
	require := require.New(t)
	type testcase struct {
		pubkeyHex string
		addresses map[xc.NativeAsset]string
	}
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset, nativeAsset.Driver()).WithNet("testnet")
		builder, err := address.NewAddressBuilder(chain.Base())
		require.NoError(err)
		for _, tc := range []testcase{
			{
				// with 0x02 prefix
				pubkeyHex: "029b6bb1f5db5dbbe7b16c686d6345a145c72cee679a0ad06f73c39861aed42492",
				addresses: map[xc.NativeAsset]string{
					xc.BTC:  "tb1qzca49vcyxkt989qcmhjfp7wyze7n9pq50k2cfd",
					xc.DOGE: "nWDiCL2RxZcMTvhUGRWCnPDWFWHSCfkhoz",
					xc.LTC:  "mhYWE7RrYCgbq4RJDaqZp8fvzVmYnPVnFD",
				},
			},
			{
				// without 0x02 prefix
				pubkeyHex: "9b6bb1f5db5dbbe7b16c686d6345a145c72cee679a0ad06f73c39861aed42492",
				addresses: map[xc.NativeAsset]string{
					xc.BTC:  "tb1qzca49vcyxkt989qcmhjfp7wyze7n9pq50k2cfd",
					xc.DOGE: "nWDiCL2RxZcMTvhUGRWCnPDWFWHSCfkhoz",
					xc.LTC:  "mhYWE7RrYCgbq4RJDaqZp8fvzVmYnPVnFD",
				},
			},
		} {
			pubkey, err := hex.DecodeString(tc.pubkeyHex)
			require.NoError(err)
			fmt.Println("checking address for ", nativeAsset)

			address, err := builder.GetAddressFromPublicKey(pubkey)
			require.NoError(err)

			expectedAddress := tc.addresses[nativeAsset]
			require.Equal(xc.Address(expectedAddress), address)
		}
	}
}

func TestGetTaprootAddressFromPublicKey(t *testing.T) {
	require := require.New(t)
	chain := xc.NewChainConfig(xc.BTC, xc.DriverBitcoin).WithNet("mainnet")

	builder, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(xc.Schnorr))

	require.NoError(err)
	pubkey, err := base64.RawStdEncoding.DecodeString("AptrsfXbXbvnsWxobWNFoUXHLO5nmgrQb3PDmGGu1CSS")
	require.NoError(err)

	address, err := builder.GetAddressFromPublicKey(pubkey)
	require.NoError(err)
	require.Equal(xc.Address("bc1pnd4mrawmtka70vtvdpkkx3dpghrjemn8ng9dqmmncwvxrtk5yjfqgd0t2x"), address)
}

func TestGetAddressFromPublicKeyUsesCompressed(t *testing.T) {
	require := require.New(t)
	chain := xc.NewChainConfig(xc.BTC, xc.DriverBitcoin).WithNet("testnet")
	builder, err := address.NewAddressBuilder(chain.Base())
	require.NoError(err)
	compressedPubkey, _ := hex.DecodeString("0228a9dd8c304464e0d0f011ca3dccb0e373afd2f5c51e89113b8be2a905687fb9")
	uncompressedPubkey, _ := hex.DecodeString("0428a9dd8c304464e0d0f011ca3dccb0e373afd2f5c51e89113b8be2a905687fb967cf9090845d6e8cac68f7bedf4335ed946c678b371c8cad7dbd5f63f1a9e992")

	addressCompressed, _ := builder.GetAddressFromPublicKey(compressedPubkey)
	addressUncompressed, _ := builder.GetAddressFromPublicKey(uncompressedPubkey)

	require.EqualValues("tb1q6y6kkfsrzhlex4u8eel436cyh26qmlmjxgwrel", addressCompressed)
	require.EqualValues("tb1q6y6kkfsrzhlex4u8eel436cyh26qmlmjxgwrel", addressUncompressed)
}
