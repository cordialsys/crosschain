package tx_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/template/tx"
	"github.com/test-go/testify/require"
)

func TestTxHash(t *testing.T) {

	tx1 := tx.Tx{}
	require.Equal(t, xc.TxHash("not implemented"), tx1.Hash())
}

func TestTxSighashes(t *testing.T) {

	tx1 := tx.Tx{}
	sighashes, err := tx1.Sighashes()
	require.NotNil(t, sighashes)
	require.EqualError(t, err, "not implemented")
}

func TestTxAddSignature(t *testing.T) {

	tx1 := tx.Tx{}
	err := tx1.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "not implemented")
}

func TestTxBody_MarshalCBOR(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: construct the receiver type.
			var tb tx.TxBody
			got, gotErr := tb.MarshalCBOR()
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("MarshalCBOR() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("MarshalCBOR() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("MarshalCBOR() = %v, want %v", got, tt.want)
			}
		})
	}
}

