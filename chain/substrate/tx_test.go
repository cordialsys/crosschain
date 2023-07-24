package substrate

import xc "github.com/jumpcrypto/crosschain"

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
