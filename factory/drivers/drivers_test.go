package drivers

import (
	"errors"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/cosmos"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/chain/solana"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/tron"

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

func (s *CrosschainTestSuite) TestAllNewClient() {
	require := s.Require()

	// server, close := testtypes.MockHTTP(&s.Suite, "{}")
	// defer close()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewClient
		if driver == xc.DriverAptos || driver == xc.DriverBitcoin || driver == xc.DriverSubstrate {
			continue
		}
		fakeAsset := &xc.ChainConfig{
			// URL:         server.URL,
			Driver: driver,
		}
		res, err := NewClient(fakeAsset, driver)
		require.NoError(err, "Missing driver for NewClient: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewTxBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewTxBuilder
		if driver == xc.DriverBitcoin {
			continue
		}
		fakeAsset := &xc.ChainConfig{
			Driver: driver,
		}
		res, err := NewTxBuilder(fakeAsset)
		require.NoError(err, "Missing driver for NewTxBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewSigner() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		fakeAsset := &xc.ChainConfig{
			Driver: driver,
		}
		res, err := NewSigner(fakeAsset)
		require.NoError(err, "Missing driver for NewSigner: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewAddressBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewAddressBuilder
		if driver == xc.DriverBitcoin {
			continue
		}
		fakeAsset := &xc.ChainConfig{
			Driver: driver,
		}
		res, err := NewAddressBuilder(fakeAsset)
		require.NoError(err, "Missing driver for NewAddressBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllCheckError() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		anError := CheckError(driver, errors.New("eof"))
		require.NotEqual(anError, xc.UnknownError, "Missing driver for CheckError: "+driver)
	}
}

func (s *CrosschainTestSuite) TestAllTxInputSerDeser() {
	require := s.Require()
	for _, driver := range xc.SupportedDrivers {
		var input xc.TxInput
		switch driver {
		case xc.DriverEVM, xc.DriverEVMLegacy:
			input = evm.NewTxInput()
		case xc.DriverCosmos, xc.DriverCosmosEvmos:
			input = cosmos.NewTxInput()
		case xc.DriverSolana:
			input = solana.NewTxInput()
		case xc.DriverAptos:
			input = aptos.NewTxInput()
		case xc.DriverBitcoin:
			input = bitcoin.NewTxInput()
		case xc.DriverSui:
			input = sui.NewTxInput()
		case xc.DriverSubstrate:
			input = substrate.NewTxInput()
		case xc.DriverTron:
			input = tron.NewTxInput()
		default:
			require.Fail("must add driver to test: " + string(driver))
		}
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
