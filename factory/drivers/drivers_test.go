package drivers_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	aptostxinput "github.com/cordialsys/crosschain/chain/aptos/tx_input"
	bitcointxinput "github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	cosmostxinput "github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	evmtxinput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/chain/evm_legacy"
	solanatxinput "github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/cordialsys/crosschain/chain/tron"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/cordialsys/crosschain/factory/drivers/registry"

	"github.com/stretchr/testify/suite"
)

// We must reference all the tx-inputs here to ensure they register their tx-inputs,
// otherwise the tests may skip compiling & loading them
var LoadedTxInputs = []xc.TxInput{
	&aptostxinput.TxInput{},
	&bitcointxinput.TxInput{},
	&cosmostxinput.TxInput{},
	&evmtxinput.TxInput{},
	&evm_legacy.TxInput{},
	&solanatxinput.TxInput{},
	&substrate.TxInput{},
	&sui.TxInput{},
	&ton.TxInput{},
	&tron.TxInput{},
}

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

		res, err := drivers.NewClient(createChainFor(driver), driver)
		require.NoError(err, "Missing driver for NewClient: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewTxInput() {
	require := s.Require()
	_, err := drivers.NewTxInput("randomthing")
	require.Error(err)

	require.GreaterOrEqual(len(registry.GetSupportedBaseTxInputs()), 1, "no registered base variants")

	for _, driver := range xc.SupportedDrivers {
		fmt.Println("driver: ", driver)
		input, err := drivers.NewTxInput(driver)
		require.NoError(err, "Missing tx-input definition: "+driver)
		require.NotNil(input)

		// no panics
		_ = input.IndependentOf(nil)
		_ = input.SafeFromDoubleSend(nil)

		// marshals
		bz, err := drivers.MarshalTxInput(input)
		require.NoError(err)

		input2, err := drivers.UnmarshalTxInput(bz)
		require.NoError(err)

		// ensure same concrete type back
		require.Equal(fmt.Sprintf("%T", input), fmt.Sprintf("%T", input2))
	}
}

func (s *CrosschainTestSuite) TestAllNewStakingInput() {
	require := s.Require()
	_, err := drivers.NewVariantInput("randomthing")
	require.Error(err)

	fmt.Println("test")
	require.GreaterOrEqual(len(registry.GetSupportedTxVariants()), 1, "no registered staking variants")

	type testcase struct {
		variants []xc.TxVariantInput
	}
	testcases := []testcase{
		{
			variants: registry.GetSupportedTxVariants(),
		},
	}

	for _, v := range testcases {
		for _, variant := range v.variants {

			require.NotEmpty(variant.GetVariant(), "must have a unique type defined")

			input, err := drivers.NewVariantInput(variant.GetVariant())
			require.NoError(err, "Missing TxInput for variant : "+variant.GetVariant())
			require.NotNil(input)

			// marshals
			bz, err := drivers.MarshalVariantInput(input)
			require.NoError(err)

			input2, err := drivers.UnmarshalVariantInput(bz)
			require.NoError(err)

			// UnmarshalTxInput should support unmarshaling variant tx inputs as they embed the base tx input
			input3, err := drivers.UnmarshalTxInput(bz)
			require.NoError(err)

			// Marshaling using the base tx input method should work
			bz2, err := drivers.MarshalTxInput(input)
			require.NoError(err)
			require.Equal(string(bz), string(bz2))

			// ensure same concrete type back
			require.Equal(fmt.Sprintf("%T", input), fmt.Sprintf("%T", input2))
			require.Equal(fmt.Sprintf("%T", input), fmt.Sprintf("%T", input3))

			inputType := strings.Split(string(variant.GetVariant()), "/")[2]

			switch inputType {
			case "staking":
				_, err := drivers.UnmarshalStakingInput(bz)
				require.NoError(err)
			case "unstaking":
				_, err := drivers.UnmarshalUnstakingInput(bz)
				require.NoError(err)
			case "withdrawing":
				_, err := drivers.UnmarshalWithdrawingInput(bz)
				require.NoError(err)
			default:
				require.Fail("unexpected txType ", inputType)
			}
		}
	}
}
func (s *CrosschainTestSuite) TestStakingVariants() {
	require := s.Require()

	variants := map[xc.TxVariantInputType]bool{}
	for _, variant := range registry.GetSupportedTxVariants() {
		variantType := variant.GetVariant()
		parts := strings.Split(string(variantType), "/")
		inputColumns := []string{"staking", "unstaking", "withdrawing"}
		require.Len(parts, 4, "variant must be in format drivers/:driver/[ "+strings.Join(inputColumns, "|")+" ]/:id")
		require.Equal("drivers", parts[0])
		require.Contains(inputColumns, parts[2], "input type column must be one of: "+strings.Join(inputColumns, ", "))
		// test driver is valid
		require.NotEmpty(xc.Driver(parts[1]).SignatureAlgorithm(), "driver is not valid")
		require.NotEmpty(parts[3], "missing ID")

		require.NotEmpty(variantType.Driver())
		require.NotEmpty(variantType.Driver().SignatureAlgorithm(), "driver is not valid")
		require.NotEmpty(variantType.Variant(), "tx variant input does not have an id / variant set.")

		if _, ok := variants[variantType]; ok {
			require.Fail("duplicate staking variant %s", variant)
		}
		variants[variantType] = true

	}
}

func (s *CrosschainTestSuite) TestAllNewTxBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		// TODO: these require custom params for NewTxBuilder
		if driver == xc.DriverBitcoin {
			continue
		}
		res, err := drivers.NewTxBuilder(createChainFor(driver))
		require.NoError(err, "Missing driver for NewTxBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllNewAddressBuilder() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		res, err := drivers.NewAddressBuilder(createChainFor(driver))
		require.NoError(err, "Missing driver for NewAddressBuilder: "+driver)
		require.NotNil(res)
	}
}

func (s *CrosschainTestSuite) TestAllCheckError() {
	require := s.Require()

	for _, driver := range xc.SupportedDrivers {
		anError := drivers.CheckError(driver, errors.New("eof"))
		require.NotEqual(anError, xclient.UnknownError, "Missing driver for CheckError: "+driver)
	}
}

func (s *CrosschainTestSuite) TestAllTxInputSerDeser() {
	require := s.Require()
	require.GreaterOrEqual(len(registry.GetSupportedBaseTxInputs()), 1, "no registered base variants")

	for _, driver := range xc.SupportedDrivers {
		var input xc.TxInput
		input, err := drivers.NewTxInput(driver)
		require.NoError(err)
		bz, err := drivers.MarshalTxInput(input)
		require.NoError(err)
		_, err = drivers.UnmarshalTxInput(bz)
		require.NoError(err)
	}
}

func (s *CrosschainTestSuite) TestSigAlg() {
	require := s.Require()
	for _, driver := range xc.SupportedDrivers {
		require.NotEmpty(driver.SignatureAlgorithm())
	}
}
