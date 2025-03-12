package bitcoin_cash_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	bitcointx "github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/stretchr/testify/require"
)

var UXTO_ASSETS []xc.NativeAsset = []xc.NativeAsset{
	xc.BCH,
}

func TestNewAddressBuilder(t *testing.T) {
	require := require.New(t)
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset)
		builder, err := bitcoin_cash.NewAddressBuilder(chain.Base())
		require.NotNil(builder)
		require.NoError(err)
	}
}

func TestGetAddressFromPublicKey(t *testing.T) {
	require := require.New(t)
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset).WithNet("testnet")
		builder, err := bitcoin_cash.NewAddressBuilder(chain.Base())
		require.NoError(err)
		_, err = base64.RawStdEncoding.DecodeString("AptrsfXbXbvnsWxobWNFoUXHLO5nmgrQb3PDmGGu1CSS")
		require.NoError(err)
		fmt.Println("checking address for ", nativeAsset)
		switch nativeAsset {
		case xc.BCH:
			pubkey_bch, err := base64.RawStdEncoding.DecodeString("A3bpQsIiW5ipniaDtYXQjeU2LwtRDkfWQNlAcY3u2pu7")
			require.NoError(err)
			address, err := builder.GetAddressFromPublicKey(pubkey_bch)
			require.NoError(err)
			require.Equal(xc.Address("bchtest:qpkxhv02hftvxe0gx654nzx3292cvfu4tqdkf49c09"), address)

		default:
			panic("need to add address test case for " + nativeAsset)
		}
	}
}

// TxBuilder

func TestNewTxBuilder(t *testing.T) {
	require := require.New(t)
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset)
		builder, err := bitcoin_cash.NewTxBuilder(chain.Base())
		require.NotNil(builder)
		require.NoError(err)
	}
}

func TestNewNativeTransfer(t *testing.T) {
	require := require.New(t)
	for _, addr := range []string{
		"tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0",
		"qzl7ex0q35q2d6aljhlhzwramp09n06fry8ssqu0qp",
		"bitcoin:qzl7ex0q35q2d6aljhlhzwramp09n06fry8ssqu0qp",
		"mqQEHYtdnjbjKTKcaGHCxFEuqwUqmSzL38",
	} {
		for _, native_asset := range []xc.NativeAsset{
			xc.BCH,
		} {
			chain := xc.NewChainConfig(native_asset).WithNet("testnet")
			builder, _ := bitcoin_cash.NewTxBuilder(chain.Base())
			from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
			to := xc.Address(addr)
			amount := xc.NewAmountBlockchainFromUint64(1)
			input := &tx_input.TxInput{
				UnspentOutputs: []tx_input.Output{{
					Value: xc.NewAmountBlockchainFromUint64(1000),
				}},
				GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
			}
			tf, err := builder.NewNativeTransfer(from, to, amount, input)
			require.NoError(err)
			require.NotNil(tf)
			hash := tf.Hash()
			require.Len(hash, 64)

			// tx must be a bitcoin cash tx
			btcTx, ok := tf.(*bitcoin_cash.Tx)
			require.True(ok)
			require.NotNil(btcTx)

			// Having not enough balance for fees will be an error
			input_small := &tx_input.TxInput{
				UnspentOutputs: []tx_input.Output{{
					Value: xc.NewAmountBlockchainFromUint64(5),
				}},
				GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
			}
			_, err = builder.NewNativeTransfer(from, to, amount, input_small)
			require.Error(err)

			// add signature
			sig := []byte{}
			for i := 0; i < 65; i++ {
				sig = append(sig, byte(i))
			}
			err = tf.AddSignatures(xc.TxSignature(sig))
			require.NoError(err)

			ser, err := tf.Serialize()
			require.NoError(err)
			require.True(len(ser) > 64)
		}
	}
}

func TestTxSighashes(t *testing.T) {
	require := require.New(t)
	tx := bitcoin_cash.Tx{
		&bitcointx.Tx{
			Input: &tx_input.TxInput{},
		},
	}
	sighashes, err := tx.Sighashes()
	require.NotNil(sighashes)
	require.NoError(err)
}
