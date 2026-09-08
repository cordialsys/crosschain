package txinputtest

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
)

// Outputs creates UTXOs and their serialized parent for client tests.
func Outputs(t testing.TB, script []byte, values ...uint64) ([]tx_input.Output, []byte) {
	t.Helper()
	parent := wire.NewMsgTx(2)
	parent.AddTxIn(wire.NewTxIn(&wire.OutPoint{}, []byte{0}, nil))
	parent.AddTxOut(wire.NewTxOut(0, []byte{0x6a}))
	for _, value := range values {
		parent.AddTxOut(wire.NewTxOut(int64(value), script))
	}
	var raw bytes.Buffer
	if err := parent.Serialize(&raw); err != nil {
		t.Fatal(err)
	}
	hash := parent.TxHash()
	outputs := make([]tx_input.Output, len(values))
	for i, value := range values {
		outputs[i] = tx_input.Output{
			Outpoint:     tx_input.Outpoint{Hash: hash[:], Index: uint32(i + 1)},
			Value:        xc.NewAmountBlockchainFromUint64(value),
			PubKeyScript: script,
		}
	}
	return outputs, raw.Bytes()
}
