package evm

import xc "github.com/cordialsys/crosschain"

func (s *CrosschainTestSuite) TestTxHashEmpty() {
	require := s.Require()
	tx := Tx{}
	require.Equal(xc.TxHash(""), tx.Hash())
}

func (s *CrosschainTestSuite) TestTxSighashesEmpty() {
	require := s.Require()
	tx := Tx{}
	sighashes, err := tx.Sighashes()
	require.NotNil(sighashes)
	require.EqualError(err, "transaction not initialized")
}

func (s *CrosschainTestSuite) TestTxAddSignatureEmpty() {
	require := s.Require()
	tx := Tx{}
	err := tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(err, "transaction not initialized")
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
