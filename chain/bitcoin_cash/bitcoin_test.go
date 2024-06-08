package bitcoin_cash

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/stretchr/testify/suite"
)

var UXTO_ASSETS []xc.NativeAsset = []xc.NativeAsset{
	xc.BCH,
}

type CrosschainTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestBitcoinTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

// Address

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	for _, nativeAsset := range UXTO_ASSETS {
		builder, err := NewAddressBuilder(&xc.ChainConfig{Chain: nativeAsset})
		require.NotNil(builder)
		require.NoError(err)
	}
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	for _, nativeAsset := range UXTO_ASSETS {
		builder, err := NewAddressBuilder(&xc.ChainConfig{
			Net:   "testnet",
			Chain: nativeAsset,
		})
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

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	for _, nativeAsset := range UXTO_ASSETS {
		builder, err := NewTxBuilder(&xc.ChainConfig{Chain: nativeAsset})
		require.NotNil(builder)
		require.NoError(err)
	}
}

func (s *CrosschainTestSuite) TestNewNativeTransfer() {
	require := s.Require()
	for _, addr := range []string{
		"tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0",
		"qzl7ex0q35q2d6aljhlhzwramp09n06fry8ssqu0qp",
		"bitcoin:qzl7ex0q35q2d6aljhlhzwramp09n06fry8ssqu0qp",
		"mqQEHYtdnjbjKTKcaGHCxFEuqwUqmSzL38",
	} {
		for _, native_asset := range []xc.NativeAsset{
			xc.BCH,
		} {
			asset := &xc.ChainConfig{Chain: native_asset, Net: "testnet"}
			builder, _ := NewTxBuilder(asset)
			from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
			to := xc.Address(addr)
			amount := xc.NewAmountBlockchainFromUint64(1)
			input := &bitcoin.TxInput{
				UnspentOutputs: []bitcoin.Output{{
					Value: xc.NewAmountBlockchainFromUint64(1000),
				}},
				GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
			}
			tf, err := builder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
			require.NoError(err)
			require.NotNil(tf)
			hash := tf.Hash()
			require.Len(hash, 64)

			// Having not enough balance for fees will be an error
			input_small := &bitcoin.TxInput{
				UnspentOutputs: []bitcoin.Output{{
					Value: xc.NewAmountBlockchainFromUint64(5),
				}},
				GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
			}
			_, err = builder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input_small)
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

// Tx

func (s *CrosschainTestSuite) TestTxHash() {
	require := s.Require()

	asset := &xc.ChainConfig{Chain: xc.BTC, Net: "testnet"}
	builder, _ := NewTxBuilder(asset)
	from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
	to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	amount := xc.NewAmountBlockchainFromUint64(1)
	input := &bitcoin.TxInput{
		UnspentOutputs: []bitcoin.Output{{
			Value: xc.NewAmountBlockchainFromUint64(1000),
		}},
		GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
	}
	tf, err := builder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.NoError(err)

	tx := tf.(*Tx)
	require.Equal(xc.TxHash("0ebdd0e519cf4bf67ac4d924c07e3312483b09844c9f16f46c04f5fe1500c788"), tx.Hash())
}

func (s *CrosschainTestSuite) TestTxSighashes() {
	require := s.Require()
	tx := Tx{
		&bitcoin.Tx{
			Input: &bitcoin.TxInput{},
		},
	}
	sighashes, err := tx.Sighashes()
	require.NotNil(sighashes)
	require.NoError(err)
}
