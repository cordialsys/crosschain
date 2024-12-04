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

func (s *CrosschainTestSuite) TestAdd() {
	require := s.Require()
	sum := NewAmountBlockchainFromStr("100000000000000000000")
	toAdd := NewAmountBlockchainFromStr("100")

	iterations := 100
	for i := 0; i < iterations; i++ {
		sum = sum.Add(&toAdd)
		require.Equal(toAdd.String(), "100")
	}

	require.Equal(sum.String(), "100000000000000010000")

	// switch
	sum = NewAmountBlockchainFromStr("100")
	toAdd = NewAmountBlockchainFromStr("100000000000000000000")

	for i := 0; i < iterations; i++ {
		sum = sum.Add(&toAdd)
		require.Equal(toAdd.String(), "100000000000000000000")
	}

	require.Equal(sum.String(), "10000000000000000000100")
}

func (s *CrosschainTestSuite) TestSub() {
	require := s.Require()
	sum := NewAmountBlockchainFromUint64(10000000)
	toDiff := NewAmountBlockchainFromUint64(100)

	iterations := 100
	for i := 0; i < iterations; i++ {
		sum = sum.Sub(&toDiff)
	}

	require.EqualValues(sum.Uint64(), 10000000-int(toDiff.Uint64())*iterations)
	require.EqualValues(toDiff.Uint64(), 100)
}

func (s *CrosschainTestSuite) TestMult() {
	require := s.Require()
	sum := NewAmountBlockchainFromUint64(1)
	toMul := NewAmountBlockchainFromUint64(100)

	iterations := 3
	for i := 0; i < iterations; i++ {
		sum = sum.Mul(&toMul)
	}

	require.EqualValues(sum.Uint64(), 1000000)
	require.EqualValues(toMul.Uint64(), 100)
}

func (s *CrosschainTestSuite) TestDiv() {
	require := s.Require()
	quot := NewAmountBlockchainFromUint64(1000000000)
	toDiv := NewAmountBlockchainFromUint64(100)

	iterations := 3
	for i := 0; i < iterations; i++ {
		quot = quot.Div(&toDiv)
	}

	require.EqualValues(quot.Uint64(), 1000)
	require.EqualValues(toDiv.Uint64(), 100)
}

func (s *CrosschainTestSuite) TestCombined() {
	// Here we chain a bunch of operations together that span many magnitudes
	// to trigger undlying slice reallocation of big integers.  The expectation
	// is that big integers are properly cloned, and do not mutate each other.
	require := s.Require()

	a := NewAmountBlockchainFromStr("10")
	b := NewAmountBlockchainFromStr("10000")
	c := NewAmountBlockchainFromStr("1000000000")
	d := NewAmountBlockchainFromStr("1000000000000000")
	e := NewAmountBlockchainFromStr("5000000000000000000000000")
	f := NewAmountBlockchainFromStr("100")

	// (a+b) = 10010
	a_plus_b := a.Add(&b)

	// c - (a + b) = 999989990
	c_minus_a_plus_b := c.Sub(&a_plus_b)

	// d * (c - (a + b)) = 999989990000000000000000
	d_times_c_minus_a_plus_b := d.Mul(&c_minus_a_plus_b)

	// e - d * (c - (a + b)) = 4000010010000000000000000
	e_minus_d_times_c_minus_a_plus_b := e.Sub(&d_times_c_minus_a_plus_b)

	// (e - d * (c - (a + b)))//f = 40000100100000000000000
	e_minus_d_times_c_minus_a_plus_b_div_f := e_minus_d_times_c_minus_a_plus_b.Div(&f)

	// (e - d * (c - (a + b)))  +  (e - d * (c - (a + b)))//f = 4040010110100000000000000
	sum_last_two := e_minus_d_times_c_minus_a_plus_b.Add(&e_minus_d_times_c_minus_a_plus_b_div_f)

	// Confirm all variables are not mutated
	require.Equal(a.String(), "10")
	require.Equal(b.String(), "10000")
	require.Equal(c.String(), "1000000000")
	require.Equal(d.String(), "1000000000000000")
	require.Equal(e.String(), "5000000000000000000000000")

	// All results + intermediates are correct
	require.Equal(a_plus_b.String(), "10010")
	require.Equal(c_minus_a_plus_b.String(), "999989990")
	require.Equal(d_times_c_minus_a_plus_b.String(), "999989990000000000000000")
	require.Equal(e_minus_d_times_c_minus_a_plus_b.String(), "4000010010000000000000000")
	require.Equal(e_minus_d_times_c_minus_a_plus_b_div_f.String(), "40000100100000000000000")
	require.Equal(sum_last_two.String(), "4040010110100000000000000")
}
