package tron

func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		Timestamp:  1000,
		Expiration: 2000,
	}
	oldInput1 := &TxInput{
		Timestamp:  100,
		Expiration: 999,
	}
	oldInput2_bad := &TxInput{
		Timestamp:  100,
		Expiration: 2001,
	}
	oldInput3_bad := &TxInput{
		Timestamp:  0,
		Expiration: 1000000,
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.SafeFromDoubleSend(oldInput2_bad))
	require.False(newInput.SafeFromDoubleSend(oldInput3_bad))

	// tron always independent
	require.True(newInput.IndependentOf(oldInput1))
	require.True(newInput.IndependentOf(oldInput2_bad))
}
