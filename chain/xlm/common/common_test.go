package common_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/common"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/require"
)

func TestGetAssetAndIssuerFromContractSuccess(t *testing.T) {
	contract := "USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"
	expected := common.AssetDetails{
		AssetCode: "USDC",
		Issuer:    xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	}

	details, err := common.GetAssetAndIssuerFromContract(contract)
	require.NoError(t, err)
	require.Equal(t, expected, details)

	native := "XLM"
	nativeDetails := common.AssetDetails{}
	details, err = common.GetAssetAndIssuerFromContract(native)
	require.NoError(t, err)
	require.Equal(t, nativeDetails, details)
}

func TestGetAssetAndIssuerFromContractErrors(t *testing.T) {
	invalidAsset := "-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"
	_, err := common.GetAssetAndIssuerFromContract(invalidAsset)
	require.ErrorContains(t, err, "invalid asset")

	invalidFormat := "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"
	_, err = common.GetAssetAndIssuerFromContract(invalidFormat)
	require.ErrorContains(t, err, "invalid format")

	invalidIssuer := "AST-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA"
	_, err = common.GetAssetAndIssuerFromContract(invalidIssuer)
	require.ErrorContains(t, err, "expected len is 56, got: 55")

	empty := ""
	_, err = common.GetAssetAndIssuerFromContract(empty)
	require.ErrorContains(t, err, "invalid format")
}

func TestCreateAlpha4AssetFromContractDetails(t *testing.T) {
	details := common.AssetDetails{
		AssetCode: "USDC",
		Issuer:    xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	}

	asset, err := common.CreateAssetFromContractDetails(details)
	require.NoError(t, err)
	require.Equal(t, asset, xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{byte('U'), byte('S'), byte('D'), byte('C')},
			Issuer:    common.MustMuxedAccountFromAddres(details.Issuer).ToAccountId(),
		},
	})
}

func TestCreateAlpha12AssetFromContractDetails(t *testing.T) {
	details := common.AssetDetails{
		AssetCode: "ValidAsset",
		Issuer:    xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	}

	asset, err := common.CreateAssetFromContractDetails(details)
	require.NoError(t, err)
	require.Equal(t, asset, xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
		AlphaNum12: &xdr.AlphaNum12{
			AssetCode: [12]byte{
				byte('V'), byte('a'), byte('l'), byte('i'),
				byte('d'), byte('A'), byte('s'), byte('s'),
				byte('e'), byte('t'), 0, 0,
			},
			Issuer: common.MustMuxedAccountFromAddres(details.Issuer).ToAccountId(),
		},
	})
}

func TestCreateInvalidAssetFromContractDetails(t *testing.T) {
	details := common.AssetDetails{
		// Asset len is greater than 12
		AssetCode: "TotallyInvalidAsset",
		Issuer:    xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	}
	_, err := common.CreateAssetFromContractDetails(details)
	require.ErrorContains(t, err, "asset code length")

	details = common.AssetDetails{
		AssetCode: "",
		Issuer:    xc.Address("GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	}

	_, err = common.CreateAssetFromContractDetails(details)
	require.ErrorContains(t, err, "asset code length")
}
