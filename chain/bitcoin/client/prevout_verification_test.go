package client_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	xc "github.com/cordialsys/crosschain"
	bitcoinclient "github.com/cordialsys/crosschain/chain/bitcoin/client"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input/txinputtest"
	"github.com/stretchr/testify/require"
)

func verifyPrevouts(chain xc.NativeAsset, outputs []tx_input.Output, script, raw []byte) error {
	return bitcoinclient.FetchLegacyPrevouts(context.Background(), chain, outputs,
		func(xc.Address) ([]byte, error) { return script, nil },
		func(context.Context, string) ([]byte, error) { return raw, nil })
}

func TestClientLegacyPrevoutVerification(t *testing.T) {
	script := append(append([]byte{0x76, 0xa9, 20}, make([]byte, 20)...), 0x88, 0xac)
	for _, chain := range []xc.NativeAsset{xc.BTC, xc.LTC, xc.DOGE} {
		for _, scenario := range []string{"valid", "understated value", "missing parent", "malformed parent", "trailing bytes", "wrong txid", "short txid", "wrong index", "wrong parent script", "spoofed witness type", "modified parent value", "parent containing witness"} {
			t.Run(fmt.Sprintf("%s/%s", chain, scenario), func(t *testing.T) {
				outputs, raw := txinputtest.Outputs(t, script, 100_000_000)
				output := &outputs[0]
				want := ""
				switch scenario {
				case "understated value":
					output.Value = xc.NewAmountBlockchainFromUint64(10_000_000)
					want = "value mismatch"
				case "missing parent":
					raw = nil
					want = "missing previous transaction"
				case "malformed parent":
					raw = []byte{1}
					want = "invalid previous transaction"
				case "trailing bytes":
					raw = append(raw, 0)
					want = "trailing data"
				case "wrong txid":
					output.Hash[0] ^= 1
					want = "txid does not match"
				case "short txid":
					output.Hash = output.Hash[:31]
					want = "invalid legacy outpoint hash"
				case "wrong index":
					output.Index = 100
					want = "index 100 is out of range"
				case "wrong parent script":
					output.Index = 0
					output.Value = xc.NewAmountBlockchainFromUint64(0)
					want = "script does not match sender"
				case "spoofed witness type":
					output.PubKeyScript = append([]byte{0, 20}, make([]byte, 20)...)
					raw = nil
					want = "missing previous transaction"
				case "modified parent value", "parent containing witness":
					var parent wire.MsgTx
					require.NoError(t, parent.Deserialize(bytes.NewReader(raw)))
					if scenario == "modified parent value" {
						parent.TxOut[output.Index].Value = 10_000_000
						output.Value = xc.NewAmountBlockchainFromUint64(10_000_000)
						want = "txid does not match"
					} else {
						parent.TxIn[0].Witness = wire.TxWitness{[]byte{1}}
					}
					var encoded bytes.Buffer
					require.NoError(t, parent.Serialize(&encoded))
					raw = encoded.Bytes()
				}
				err := verifyPrevouts(chain, outputs, script, raw)
				if want != "" {
					require.ErrorContains(t, err, want)
				} else {
					require.NoError(t, err)
					require.Equal(t, script, output.PubKeyScript)
				}
			})
		}
	}
}

func TestClientPreviousTxAmountBounds(t *testing.T) {
	for _, tc := range []struct {
		chain xc.NativeAsset
		max   uint64
	}{{xc.BTC, 21_000_000 * 100_000_000}, {xc.LTC, 84_000_000 * 100_000_000}, {xc.DOGE, 10_000_000_000 * 100_000_000}} {
		for _, value := range []uint64{tc.max, tc.max + 1, ^uint64(0)} {
			outputs, raw := txinputtest.Outputs(t, []byte{0x51}, value)
			err := verifyPrevouts(tc.chain, outputs, []byte{0x51}, raw)
			if value == tc.max {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "invalid previous transaction output value")
			}
		}
	}
}

func TestClientLegacyPrevoutScope(t *testing.T) {
	for _, chain := range []xc.NativeAsset{xc.BTC, xc.LTC, xc.DOGE} {
		for _, script := range [][]byte{append([]byte{0, 20}, make([]byte, 20)...), append([]byte{0x51, 32}, make([]byte, 32)...)} {
			outputs := []tx_input.Output{{Outpoint: tx_input.Outpoint{Hash: make([]byte, 32)}}}
			err := verifyPrevouts(chain, outputs, script, nil)
			if chain == xc.DOGE {
				require.ErrorContains(t, err, "missing previous transaction")
			} else {
				require.NoError(t, err)
				require.Equal(t, script, outputs[0].PubKeyScript)
			}
		}
	}
	for _, chain := range []xc.NativeAsset{xc.BCH, xc.ZEC, xc.DASH} {
		require.False(t, bitcoinclient.SupportsLegacyPrevoutVerification(chain))
	}
}

func TestClientDogecoinP2PKOwnership(t *testing.T) {
	_, key := btcec.PrivKeyFromBytes([]byte{1})
	for _, publicKey := range [][]byte{key.SerializeCompressed(), key.SerializeUncompressed()} {
		script, err := txscript.NewScriptBuilder().AddData(publicKey).AddOp(txscript.OP_CHECKSIG).Script()
		require.NoError(t, err)
		expected := append(append([]byte{0x76, 0xa9, 20}, btcutil.Hash160(publicKey)...), 0x88, 0xac)
		outputs, raw := txinputtest.Outputs(t, script, 100_000_000)
		require.NoError(t, verifyPrevouts(xc.DOGE, outputs, expected, raw))
		expected[3] ^= 1
		require.ErrorContains(t, verifyPrevouts(xc.DOGE, outputs, expected, raw), "script does not match sender")
	}
}
