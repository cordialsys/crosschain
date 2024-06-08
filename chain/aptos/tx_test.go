package aptos

func (s *AptosTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		SequenceNumber: 10,
	}
	oldInput1 := &TxInput{
		SequenceNumber: 10,
	}
	oldInput2_bad := &TxInput{
		SequenceNumber: 11,
	}
	oldInput3_bad := &TxInput{
		SequenceNumber: 9,
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.SafeFromDoubleSend(oldInput2_bad))
	require.False(newInput.SafeFromDoubleSend(oldInput3_bad))
}
