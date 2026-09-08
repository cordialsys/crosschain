package builder_test

import (
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/stretchr/testify/require"
)

// Legacy builders trust the caller's UTXO data; parent verification is performed
// by the online client and is not repeated after offline input transport.
func TestLegacyBuilderAcceptsInputsWithoutParents(t *testing.T) {
	for _, native := range []xc.NativeAsset{xc.BTC, xc.LTC, xc.DOGE} {
		t.Run(string(native), func(t *testing.T) {
			chain := xc.NewChainConfig(native).WithNet("testnet")
			b, err := builder.NewTxBuilder(chain.Base())
			require.NoError(t, err)
			addr, err := btcutil.NewAddressPubKeyHash(make([]byte, 20), b.Params)
			require.NoError(t, err)
			script, err := txscript.PayToAddrScript(addr)
			require.NoError(t, err)
			from := xc.Address(addr.EncodeAddress())
			input := tx_input.TxInput{
				Address: from,
				UnspentOutputs: []tx_input.Output{{
					Outpoint:     tx_input.Outpoint{Hash: make([]byte, 32), Index: 1},
					Value:        xc.NewAmountBlockchainFromUint64(100_000_000),
					PubKeyScript: script,
				}},
				GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(1),
			}
			data, err := json.Marshal(input)
			require.NoError(t, err)
			require.NotContains(t, string(data), "previous_tx")
			var offline tx_input.TxInput
			require.NoError(t, json.Unmarshal(data, &offline))
			amount := xc.NewAmountBlockchainFromUint64(1_000_000)
			built, err := b.NewNativeTransfer(from, from, amount, &offline)
			require.NoError(t, err)
			require.EqualValues(t, 98_999_745, built.(*tx.Tx).MsgTx.TxOut[1].Value)
			_, err = built.Sighashes()
			require.NoError(t, err)

			args, err := xcbuilder.NewMultiTransferArgs(chain.Base(),
				[]*xcbuilder.Sender{buildertest.MustNewSender(from, nil)},
				[]*xcbuilder.Receiver{buildertest.MustNewReceiver(from, amount)})
			require.NoError(t, err)
			_, err = b.MultiTransfer(*args, &tx_input.MultiTransferInput{Inputs: []tx_input.TxInput{offline}})
			require.NoError(t, err)
		})
	}
}
