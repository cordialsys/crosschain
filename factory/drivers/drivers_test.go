package drivers

import (
	"errors"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"

	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
	TestNativeAssets []xc.NativeAsset
}

func (s *CrosschainTestSuite) SetupTest() {
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func createChainFor(driver xc.Driver) *xc.ChainConfig {
	fakeAsset := &xc.ChainConfig{
		// URL:         server.URL,
		Driver: driver,
	}
	if driver == xc.DriverBitcoin {
		fakeAsset.Chain = "BTC"
		fakeAsset.AuthSecret = "1234"
	}
	if driver == xc.DriverBitcoinLegacy {
		fakeAsset.Chain = "DOGE"
		fakeAsset.AuthSecret = "1234"
	}
	if driver == xc.DriverBitcoinCash {
		fakeAsset.Chain = "BCH"
		fakeAsset.AuthSecret = "1234"
	}
	if driver == xc.DriverSubstrate {
		fakeAsset.ChainPrefix = "0"
	}
	return fakeAsset
}

func (s *CrosschainTestSuite) TestAllNewClient() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewClient
		if driver == xc.DriverAptos || driver == xc.DriverSubstrate {
			continue
		}

		res, err := NewClient(createChainFor(driver), driver)
		require.NoError(err, "Missing driver for NewClient: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewTxInput() {
	require := s.Require()
	_, err := NewTxInput("randomthing")
	require.Error(err)

	for _, driver := range xc.SupportedDrivers {
		input, err := NewTxInput(driver)
		require.NoError(err, "Missing driver for NewClient: "+driver)
		require.NotNil(input)

		// no panics
		_ = input.IndependentOf(nil)
		_ = input.SafeFromDoubleSend(nil)

		// marshals
		bz, err := MarshalTxInput(input)
		require.NoError(err)

		input2, err := UnmarshalTxInput(bz)
		require.NoError(err)

		// ensure same concrete type back
		require.Equal(fmt.Sprintf("%T", input), fmt.Sprintf("%T", input2))
	}
}

func (s *CrosschainTestSuite) TestAllNewStakingInput() {
	require := s.Require()
	_, err := NewVariantInput("randomthing")
	require.Error(err)

	type testcase struct {
		variants []xc.TxVariant
		txType   string
	}
	testcases := []testcase{
		{
			variants: xc.SupportedStakingVariants,
			txType:   "staking",
		},
		{
			variants: xc.SupportedUnstakingVariants,
			txType:   "unstaking",
		},
	}

	for _, v := range testcases {
		for _, variant := range v.variants {

			require.Equal(v.txType, variant.TxType())

			input, err := NewVariantInput(variant)
			require.NoError(err, "Missing TxInput for variant : "+variant)
			require.NotNil(input)

			// marshals
			bz, err := MarshalVariantInput(input)
			require.NoError(err)

			input2, err := UnmarshalVariantInput(bz)
			require.NoError(err)

			// ensure same concrete type back
			require.Equal(fmt.Sprintf("%T", input), fmt.Sprintf("%T", input2))

			switch v.txType {
			case "staking":
				_, err := UnmarshalStakingInput(bz)
				require.NoError(err)
			case "unstaking":
				_, err := UnmarshalUnstakingInput(bz)
				require.NoError(err)
			default:
				require.Fail("unexpected txType ", v.txType)
			}
		}
	}
}

func (s *CrosschainTestSuite) TestAllNewTxBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewTxBuilder
		if driver == xc.DriverBitcoin {
			continue
		}
		res, err := NewTxBuilder(createChainFor(driver))
		require.NoError(err, "Missing driver for NewTxBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewAddressBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		res, err := NewAddressBuilder(createChainFor(driver))
		require.NoError(err, "Missing driver for NewAddressBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllCheckError() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		anError := CheckError(driver, errors.New("eof"))
		require.NotEqual(anError, xclient.UnknownError, "Missing driver for CheckError: "+driver)
	}
}

func (s *CrosschainTestSuite) TestAllTxInputSerDeser() {
	require := s.Require()
	for _, driver := range xc.SupportedDrivers {
		var input xc.TxInput
		input, err := NewTxInput(driver)
		require.NoError(err)
		bz, err := MarshalTxInput(input)
		require.NoError(err)
		_, err = UnmarshalTxInput(bz)
		// output, err := UnmarshalTxInput(bz)
		require.NoError(err)
		// require.Equal(input, output)
	}
}

func (s *CrosschainTestSuite) TestSigAlg() {
	require := s.Require()
	for _, driver := range xc.SupportedDrivers {
		require.NotEmpty(driver.SignatureAlgorithm())
	}
}
