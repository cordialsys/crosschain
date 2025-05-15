package tx_input_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestSafeFromDoubleSpend(t *testing.T) {
	vectors := []struct {
		nonce1              uint64
		nonce2              uint64
		safeFromDoubleSpend bool
		independentOf       bool
	}{
		{
			nonce1:              1,
			nonce2:              2,
			safeFromDoubleSpend: false,
			independentOf:       true,
		},
		{
			nonce1:              1,
			nonce2:              1,
			safeFromDoubleSpend: true,
			independentOf:       false,
		},
	}
	for _, v := range vectors {
		newInput := &TxInput{
			Nonce: v.nonce1,
		}
		oldInput := &TxInput{
			Nonce: v.nonce2,
		}

		require.Equal(t, v.safeFromDoubleSpend, newInput.SafeFromDoubleSend(oldInput))
		require.Equal(t, v.independentOf, newInput.IndependentOf(oldInput))
	}
}
