package substrate

import xc "github.com/cordialsys/crosschain"

func (s *CrosschainTestSuite) TestTx() {
	// TODO: write tests
	require := s.Require()
	builder, err := NewTxBuilder(&xc.TaskConfig{})
	require.Nil(err)
	txInput := NewTxInput()

	builder.NewTransfer(
		xc.Address("5D4ZHEUiqCnpPBBizYSRJ1wDhdEfxHvoe4ogoUep636kgDVW"),
		xc.Address("5D9tgcUkKnDw5viByT7Hujj7xrbuM2mVM4Rtn1Bky6nJSFjD"),
		xc.NewAmountBlockchainFromUint64(1000000),
		txInput,
	)
}

func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		Nonce: 10,
	}
	oldInput1 := &TxInput{
		Nonce: 10,
	}
	oldInput2_bad := &TxInput{
		Nonce: 11,
	}
	oldInput3_bad := &TxInput{
		Nonce: 9,
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.SafeFromDoubleSend(oldInput2_bad))
	require.False(newInput.SafeFromDoubleSend(oldInput3_bad))

	require.False(newInput.IndependentOf(oldInput1))
	require.True(newInput.IndependentOf(oldInput2_bad))
}
