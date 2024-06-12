package crosschain_test

import (
	. "github.com/cordialsys/crosschain"
	"github.com/shopspring/decimal"
)

func (s *CrosschainTestSuite) TestNewAmountBlockchainFromUint64() {
	require := s.Require()
	amount := NewAmountBlockchainFromUint64(123)
	require.NotNil(amount)
	require.Equal(amount.Uint64(), uint64(123))
	require.Equal(amount.String(), "123")
}

func (s *CrosschainTestSuite) TestNewAmountBlockchainFromFloat64() {
	require := s.Require()
	amount := NewAmountBlockchainToMaskFloat64(1.23)
	require.NotNil(amount)
	require.Equal(amount.Uint64(), uint64(1230000))
	require.Equal(amount.String(), "1230000")

	amountFloat := amount.UnmaskFloat64()
	require.Equal(amountFloat, 1.23)
}

func (s *CrosschainTestSuite) TestAmountHumanReadable() {
	require := s.Require()
	amountDec, _ := decimal.NewFromString("10.3")
	amount := AmountHumanReadable(amountDec)
	require.NotNil(amount)
	require.Equal(amount.String(), "10.3")
}

func (s *CrosschainTestSuite) TestNewAmountHumanReadableFromStr() {
	require := s.Require()
	amount, err := NewAmountHumanReadableFromStr("10.3")
	require.NoError(err)
	require.NotNil(amount)
	require.Equal(amount.String(), "10.3")

	amount, err = NewAmountHumanReadableFromStr("0")
	require.NoError(err)
	require.NotNil(amount)
	require.Equal(amount.String(), "0")

	amount, err = NewAmountHumanReadableFromStr("")
	require.Error(err)
	require.NotNil(amount)
	require.Equal(amount.String(), "0")

	amount, err = NewAmountHumanReadableFromStr("invalid")
	require.Error(err)
	require.NotNil(amount)
	require.Equal(amount.String(), "0")
}

func (s *CrosschainTestSuite) TestNewBlockchainAmountStr() {
	require := s.Require()
	amount := NewAmountBlockchainFromStr("10")
	require.EqualValues(amount.Uint64(), 10)

	amount = NewAmountBlockchainFromStr("10.1")
	require.EqualValues(amount.Uint64(), 0)

	amount = NewAmountBlockchainFromStr("0x10")
	require.EqualValues(amount.Uint64(), 16)
}

func (s *CrosschainTestSuite) TestLegacyGasCalculation() {
	require := s.Require()

	// Multiplier should default to 1
	require.EqualValues(
		1000,
		NewAmountBlockchainFromUint64(1000).ApplyGasPriceMultiplier(&ChainConfig{}).Uint64(),
	)
	require.EqualValues(
		1200,
		NewAmountBlockchainFromUint64(1000).ApplyGasPriceMultiplier(&ChainConfig{ChainGasMultiplier: 1.2}).Uint64(),
	)
	require.EqualValues(
		500,
		NewAmountBlockchainFromUint64(1000).ApplyGasPriceMultiplier(&ChainConfig{ChainGasMultiplier: .5}).Uint64(),
	)
	require.EqualValues(
		1500,
		NewAmountBlockchainFromUint64(1000).ApplyGasPriceMultiplier(&ChainConfig{ChainGasMultiplier: 1.5}).Uint64(),
	)
}
