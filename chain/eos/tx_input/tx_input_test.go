package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestSafeFromDoubleSpend(t *testing.T) {

	newInput := &TxInput{}
	oldInput1 := &TxInput{}
	// Defaults are false but each chain has conditions
	require.False(t, newInput.SafeFromDoubleSend(oldInput1))
	// EOS is always independent
	require.True(t, newInput.IndependentOf(oldInput1))
}

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		newInput xc.TxInput
		oldInput xc.TxInput

		doubleSpendSafe bool
	}
	now := time.Now()
	vectors := []testcase{
		{
			newInput:        &TxInput{},
			oldInput:        &TxInput{},
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{},
			// check no old input
			oldInput:        nil,
			doubleSpendSafe: false,
		},
		{
			oldInput: &TxInput{
				Timestamp:   now.Unix(),
				HeadBlockID: []byte{1},
			},
			newInput: &TxInput{
				Timestamp:   now.Unix(),
				HeadBlockID: []byte{2},
			},

			doubleSpendSafe: false,
		},
		{
			oldInput: &TxInput{
				Timestamp:   now.Add(-tx_input.ExpirationPeriod).Unix(),
				HeadBlockID: []byte{1},
			},
			newInput: &TxInput{
				Timestamp:   now.Unix(),
				HeadBlockID: []byte{2},
			},

			doubleSpendSafe: false,
		},
		{
			oldInput: &TxInput{
				Timestamp:   now.Add(-(tx_input.ExpirationPeriod + tx_input.ExpirationGracePeriod + 1*time.Second)).Unix(),
				HeadBlockID: []byte{1},
			},
			newInput: &TxInput{
				Timestamp:   now.Unix(),
				HeadBlockID: []byte{2},
			},

			doubleSpendSafe: true,
		},
		{
			// not double spend because blockID is the same
			oldInput: &TxInput{
				Timestamp:   now.Add(-(tx_input.ExpirationPeriod + tx_input.ExpirationGracePeriod + 100*time.Second)).Unix(),
				HeadBlockID: []byte{1},
			},
			newInput: &TxInput{
				Timestamp:   now.Unix(),
				HeadBlockID: []byte{1},
			},

			doubleSpendSafe: false,
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("testcase_%d", i), func(t *testing.T) {
			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			t.Logf("testcase %d - expect safe=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, string(newBz), string(oldBz))
			require.True(t, v.newInput.IndependentOf(v.oldInput), "EOS is always independent")
			require.Equal(t,
				v.doubleSpendSafe,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				"SafeFromDoubleSend",
			)
		})
	}
}
