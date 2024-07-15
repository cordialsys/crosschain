package substrate_test

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
)

func (s *CrosschainTestSuite) TestTx() {
	// TODO: write tests
	require := s.Require()
	builder, err := substrate.NewTxBuilder(&xc.TaskConfig{})
	require.NoError(err)
	txInput := substrate.NewTxInput()

	builder.NewTransfer(
		xc.Address("5D4ZHEUiqCnpPBBizYSRJ1wDhdEfxHvoe4ogoUep636kgDVW"),
		xc.Address("5D9tgcUkKnDw5viByT7Hujj7xrbuM2mVM4Rtn1Bky6nJSFjD"),
		xc.NewAmountBlockchainFromUint64(1000000),
		txInput,
	)
}
