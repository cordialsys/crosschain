package evm

func (s *CrosschainTestSuite) TestGweiArthmetic() {
	require := s.Require()
	gwei := 3
	wei := GweiToWei(uint64(gwei))
	require.EqualValues(3000000000, wei.Uint64())

	gwei = 5
	wei = GweiToWei(uint64(gwei))
	require.EqualValues(5000000000, wei.Uint64())
}
