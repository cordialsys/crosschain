package cosmos

import (
	xc "github.com/jumpcrypto/crosschain"
)

func (s *CrosschainTestSuite) TestTax() {
	require := s.Require()
	amount := xc.NewAmountBlockchainFromUint64(100)
	tax := 0.05
	require.Equal(xc.NewAmountBlockchainFromUint64(5), GetTaxFrom(amount, tax))

	tax = 0.000000001
	require.Equal(xc.NewAmountBlockchainFromUint64(0), GetTaxFrom(amount, tax))
}
