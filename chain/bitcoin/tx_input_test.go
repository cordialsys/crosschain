package bitcoin

func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		UnspentOutputs: []Output{
			{Outpoint: Outpoint{Hash: []byte{10}, Index: 10}},
			{Outpoint: Outpoint{Hash: []byte{12}, Index: 12}},
		},
	}
	oldInput1 := &TxInput{
		UnspentOutputs: []Output{
			{Outpoint: Outpoint{Hash: []byte{10}, Index: 10}},
			{Outpoint: Outpoint{Hash: []byte{11}, Index: 11}},
			{Outpoint: Outpoint{Hash: []byte{13}, Index: 13}},
		},
	}
	oldInput2_bad := &TxInput{
		UnspentOutputs: []Output{
			{Outpoint: Outpoint{Hash: []byte{11}, Index: 11}},
			{Outpoint: Outpoint{Hash: []byte{13}, Index: 13}},
		},
	}
	oldInput3_bad := &TxInput{
		UnspentOutputs: []Output{
			{Outpoint: Outpoint{Hash: []byte{11}, Index: 11}},
		},
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.SafeFromDoubleSend(oldInput2_bad))
	require.False(newInput.SafeFromDoubleSend(oldInput3_bad))

	require.False(newInput.IndependentOf(oldInput1))
	require.True(newInput.IndependentOf(oldInput3_bad))
}
