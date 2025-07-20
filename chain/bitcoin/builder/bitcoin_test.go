package builder_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin/address"
	. "github.com/cordialsys/crosschain/chain/bitcoin/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var UXTO_ASSETS []xc.NativeAsset = []xc.NativeAsset{
	xc.BTC,
	xc.DOGE,
	xc.LTC,
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
		chain := xc.NewChainConfig(nativeAsset)
		builder, err := address.NewAddressBuilder(chain.Base())
		require.NotNil(builder)
		require.NoError(err)
	}
}

func (s *CrosschainTestSuite) TestNewAddressBuilderInvalidAlgorithm() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC)
	_, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(xc.Ed255))
	require.ErrorContains(err, "ed255")
}

func (s *CrosschainTestSuite) TestNewAddressBuilderValidAlgorithms() {
	require := s.Require()
	tests := []struct {
		name                string
		asset               xc.NativeAsset
		algorithm           xc.SignatureType
		expectedAddressType address.AddressType
	}{
		{
			name:                "taproot-address",
			asset:               xc.BTC,
			algorithm:           xc.Schnorr,
			expectedAddressType: address.AddressTypeTaproot,
		},
		{
			name:                "legacy-address",
			asset:               xc.DOGE,
			algorithm:           xc.K256Sha256,
			expectedAddressType: address.AddressTypeLegacy,
		},
		{
			name:                "segwit-explicit-algo-address",
			asset:               xc.BTC,
			algorithm:           xc.K256Sha256,
			expectedAddressType: address.AddressTypeSegWit,
		},
		{
			name:                "segwit-missing-algo-address",
			asset:               xc.BTC,
			algorithm:           "",
			expectedAddressType: address.AddressTypeSegWit,
		},
	}

	for _, t := range tests {
		s.Run(t.name, func() {
			chain := xc.NewChainConfig(t.asset, t.asset.Driver())
			builder, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(t.algorithm))
			require.NoError(err)

			addressType, err := builder.(address.AddressBuilder).GetAddressType()
			require.Equal(addressType, t.expectedAddressType)
		})
	}
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	type testcase struct {
		pubkeyHex string
		addresses map[xc.NativeAsset]string
	}
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset, nativeAsset.Driver()).WithNet("testnet")
		builder, err := address.NewAddressBuilder(chain.Base())
		require.NoError(err)
		for _, tc := range []testcase{
			{
				// with 0x02 prefix
				pubkeyHex: "029b6bb1f5db5dbbe7b16c686d6345a145c72cee679a0ad06f73c39861aed42492",
				addresses: map[xc.NativeAsset]string{
					xc.BTC:  "tb1qzca49vcyxkt989qcmhjfp7wyze7n9pq50k2cfd",
					xc.DOGE: "nWDiCL2RxZcMTvhUGRWCnPDWFWHSCfkhoz",
					xc.LTC:  "mhYWE7RrYCgbq4RJDaqZp8fvzVmYnPVnFD",
				},
			},
			{
				// without 0x02 prefix
				pubkeyHex: "9b6bb1f5db5dbbe7b16c686d6345a145c72cee679a0ad06f73c39861aed42492",
				addresses: map[xc.NativeAsset]string{
					xc.BTC:  "tb1qzca49vcyxkt989qcmhjfp7wyze7n9pq50k2cfd",
					xc.DOGE: "nWDiCL2RxZcMTvhUGRWCnPDWFWHSCfkhoz",
					xc.LTC:  "mhYWE7RrYCgbq4RJDaqZp8fvzVmYnPVnFD",
				},
			},
		} {
			pubkey, err := hex.DecodeString(tc.pubkeyHex)
			require.NoError(err)
			fmt.Println("checking address for ", nativeAsset)

			address, err := builder.GetAddressFromPublicKey(pubkey)
			require.NoError(err)

			expectedAddress := tc.addresses[nativeAsset]
			require.Equal(xc.Address(expectedAddress), address)
		}
	}
}

func (s *CrosschainTestSuite) TestGetTaprootAddressFromPublicKey() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC, xc.DriverBitcoin).WithNet("mainnet")

	builder, err := address.NewAddressBuilder(chain.Base(), xcaddress.OptionAlgorithm(xc.Schnorr))

	require.NoError(err)
	pubkey, err := base64.RawStdEncoding.DecodeString("AptrsfXbXbvnsWxobWNFoUXHLO5nmgrQb3PDmGGu1CSS")
	require.NoError(err)

	address, err := builder.GetAddressFromPublicKey(pubkey)
	require.NoError(err)
	require.Equal(xc.Address("bc1pnd4mrawmtka70vtvdpkkx3dpghrjemn8ng9dqmmncwvxrtk5yjfqgd0t2x"), address)
}

func (s *CrosschainTestSuite) TestGetAddressFromPublicKeyUsesCompressed() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC, xc.DriverBitcoin).WithNet("testnet")
	builder, err := address.NewAddressBuilder(chain.Base())
	require.NoError(err)
	compressedPubkey, _ := hex.DecodeString("0228a9dd8c304464e0d0f011ca3dccb0e373afd2f5c51e89113b8be2a905687fb9")
	uncompressedPubkey, _ := hex.DecodeString("0428a9dd8c304464e0d0f011ca3dccb0e373afd2f5c51e89113b8be2a905687fb967cf9090845d6e8cac68f7bedf4335ed946c678b371c8cad7dbd5f63f1a9e992")

	addressCompressed, _ := builder.GetAddressFromPublicKey(compressedPubkey)
	addressUncompressed, _ := builder.GetAddressFromPublicKey(uncompressedPubkey)

	require.EqualValues("tb1q6y6kkfsrzhlex4u8eel436cyh26qmlmjxgwrel", addressCompressed)
	require.EqualValues("tb1q6y6kkfsrzhlex4u8eel436cyh26qmlmjxgwrel", addressUncompressed)
}

func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC, xc.BTC.Driver()).WithNet("testnet")
	builderI, err := address.NewAddressBuilder(chain.Base())
	builder := builderI.(address.AddressBuilder)
	require.NoError(err)
	pubkey, err := base64.RawStdEncoding.DecodeString("AptrsfXbXbvnsWxobWNFoUXHLO5nmgrQb3PDmGGu1CSS")
	require.NoError(err)
	addresses, err := builder.GetAllPossibleAddressesFromPublicKey(pubkey)
	require.NoError(err)

	validated_p2pkh := false
	validated_p2wkh := false

	fmt.Println(addresses)
	for _, addr := range addresses {
		if addr.Address == "mhYWE7RrYCgbq4RJDaqZp8fvzVmYnPVnFD" {
			require.Equal(xc.AddressTypeP2PKH, addr.Type)
			validated_p2pkh = true
		} else if addr.Address == "tb1qzca49vcyxkt989qcmhjfp7wyze7n9pq50k2cfd" {
			require.Equal(xc.AddressTypeP2WPKH, addr.Type)
			validated_p2wkh = true
		} else {
			// panic("unexpected address generated: " + addr.Address)
		}
	}
	require.True(validated_p2pkh)
	require.True(validated_p2wkh)
}

// TxBuilder

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	for _, nativeAsset := range UXTO_ASSETS {
		chain := xc.NewChainConfig(nativeAsset)
		builder, err := NewTxBuilder(chain.Base())
		require.NotNil(builder)
		require.NoError(err)
	}
}

func (s *CrosschainTestSuite) TestNewNativeTransfer() {
	require := s.Require()
	for _, fromAddr := range []string{
		// legacy
		"mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6",
		// segwit
		"tb1qhymp5maj7x2rqxsj02exqn26v5jcqm0q3x3pz4",
		// taproot (not supported)
		// "tb1p5gkytm46mtksmssryta62fejfxvh82vnqs96hnd96gwmn0ztz4esam80dt",
	} {
		for _, toAddr := range []string{
			// legacy
			"mxVFsFW5N4mu1HPkxPttorvocvzeZ7KZyk",
			// segwit
			"tb1qtguj96eqjtzt2fywyqdgmuw6wtpdsuahheqja6",
			// taproot
			"tb1p5gkytm46mtksmssryta62fejfxvh82vnqs96hnd96gwmn0ztz4esam80dt",
		} {
			for _, native_asset := range []xc.NativeAsset{
				xc.BTC,
			} {
				chain := xc.NewChainConfig(native_asset).WithNet("testnet")
				builder, _ := NewTxBuilder(chain.Base())
				from := xc.Address(fromAddr)
				to := xc.Address(toAddr)
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
				err = tf.SetSignatures(&xc.SignatureResponse{
					Signature: sig,
				})
				require.NoError(err)

				ser, err := tf.Serialize()
				require.NoError(err)
				require.True(len(ser) > 64)
			}
		}
	}
}

func (s *CrosschainTestSuite) TestNewTokenTransfer() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	builder, _ := NewTxBuilder(chain.Base())
	from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
	to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	amount := xc.NewAmountBlockchainFromUint64(1)
	input := &tx_input.TxInput{
		UnspentOutputs: []tx_input.Output{{
			Value: xc.NewAmountBlockchainFromUint64(1000),
		}},
		GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
	}
	args, err := xcbuilder.NewTransferArgs(from, to, amount, xcbuilder.OptionContractAddress("1234"))
	require.NoError(err)
	_, err = builder.Transfer(args, input)
	require.ErrorContains(err, "token transfers are not supported")
}

func (s *CrosschainTestSuite) TestNewTransfer() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	builder, _ := NewTxBuilder(chain.Base())
	from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
	to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	amount := xc.NewAmountBlockchainFromUint64(1)
	input := &tx_input.TxInput{
		UnspentOutputs: []tx_input.Output{{
			Value: xc.NewAmountBlockchainFromUint64(1000),
		}},
		GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
	}
	args, err := xcbuilder.NewTransferArgs(from, to, amount)
	require.NoError(err)
	tf, err := builder.Transfer(args, input)
	require.NotNil(tf)
	require.NoError(err)
}

// Tx

func (s *CrosschainTestSuite) TestTxHash() {
	require := s.Require()

	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	builder, _ := NewTxBuilder(chain.Base())
	from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
	to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	amount := xc.NewAmountBlockchainFromUint64(1)
	input := &tx_input.TxInput{
		UnspentOutputs: []tx_input.Output{{
			Value: xc.NewAmountBlockchainFromUint64(1000),
		}},
		GasPricePerByte: xc.NewAmountBlockchainFromUint64(1),
	}
	tf, err := builder.NewNativeTransfer(from, to, amount, input)
	require.NoError(err)

	tx := tf.(*tx.Tx)
	require.Equal(xc.TxHash("0ebdd0e519cf4bf67ac4d924c07e3312483b09844c9f16f46c04f5fe1500c788"), tx.Hash())
}

func (s *CrosschainTestSuite) TestTxSighashes() {
	require := s.Require()
	tx := tx.Tx{
		UnspentOutputs: []tx_input.Output{},
	}
	sighashes, err := tx.Sighashes()
	require.NotNil(sighashes)
	require.NoError(err)
}

func (s *CrosschainTestSuite) TestTxAddSignature() {
	require := s.Require()
	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	builder, _ := NewTxBuilder(chain.Base())
	from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
	to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{
		UnspentOutputs: []tx_input.Output{{
			Value: xc.NewAmountBlockchainFromUint64(1000),
		}},
	}
	tf, err := builder.NewNativeTransfer(from, to, amount, input)
	require.NoError(err)

	txObject := tf.(*tx.Tx)
	err = txObject.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: []byte{1, 2, 3, 4},
		},
	}...)
	require.ErrorContains(err, "signature must be 64 or 65 length")
	sig := []byte{}
	for i := 0; i < 65; i++ {
		sig = append(sig, byte(i))
	}
	err = txObject.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: sig,
		},
	}...)
	require.NoError(err)

	// can't sign multiple times in a row
	err = txObject.SetSignatures([]*xc.SignatureResponse{
		{
			Signature: sig,
		},
	}...)
	require.ErrorContains(err, "already signed")

	// must have a signature for each input needed
	tf, _ = builder.NewNativeTransfer(from, to, amount, input)
	err = tf.(*tx.Tx).SetSignatures([]*xc.SignatureResponse{
		{
			Signature: sig,
		},
		{
			Signature: sig,
		},
	}...)
	require.ErrorContains(err, "expected 1 signatures, got 2 signatures")

	// 2 inputs = 2 sigs
	amount = xc.NewAmountBlockchainFromUint64(15000)
	input = &tx_input.TxInput{
		UnspentOutputs: []tx_input.Output{{
			Value: xc.NewAmountBlockchainFromUint64(10000),
		},
			{
				Value: xc.NewAmountBlockchainFromUint64(10000),
			},
		},
	}
	tf, _ = builder.NewNativeTransfer(from, to, amount, input)
	require.Len(tf.(*tx.Tx).UnspentOutputs, 2)
	err = tf.(*tx.Tx).SetSignatures([]*xc.SignatureResponse{
		{
			Signature: sig,
		},
		{
			Signature: sig,
		},
	}...)
	require.NoError(err)
}

func genInput(addr string, totalAmount int, numberUtxos int) tx_input.TxInput {
	input := tx_input.TxInput{
		Address:        xc.Address(addr),
		UnspentOutputs: []tx_input.Output{},
	}
	if numberUtxos <= 0 {
		return input
	}
	// add (i-1) utxos of 1 satoshi each
	for range numberUtxos - 1 {
		input.UnspentOutputs = append(input.UnspentOutputs, tx_input.Output{
			Value: xc.NewAmountBlockchainFromUint64(1),
		})
	}
	// add the last utxo with the remaining amount
	input.UnspentOutputs = append(input.UnspentOutputs, tx_input.Output{
		Value: xc.NewAmountBlockchainFromUint64(uint64(totalAmount - (numberUtxos - 1))),
	})
	return input
}

func TestMultiTransferChange(t *testing.T) {
	require := require.New(t)
	chain := xc.NewChainConfig(xc.BTC).WithNet("testnet")
	txBuilder, _ := NewTxBuilder(chain.Base())

	type testcase struct {
		name       string
		fromInputs []tx_input.TxInput
		froms      []string
		tos        []tx.Recipient
		// should be the same as the tos but with change address recipients added.
		expectedRecipients []tx.Recipient
		totalUtxoSpend     int
	}

	tests := []testcase{
		{
			name: "single spender",
			fromInputs: []tx_input.TxInput{
				genInput("tb1qlcpaaqyfqqraajlp7s4j4h8quuk07lnys2myvw", 10000, 5),
			},
			totalUtxoSpend: 5,
			tos: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(3000),
				},
			},
			expectedRecipients: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(3000),
				},
				{
					// change
					To:    xc.Address("tb1qlcpaaqyfqqraajlp7s4j4h8quuk07lnys2myvw"),
					Value: xc.NewAmountBlockchainFromUint64(7000),
				},
			},
		},

		{
			name: "multiple spenders middle address gets change",
			fromInputs: []tx_input.TxInput{
				genInput("tb1qlcpaaqyfqqraajlp7s4j4h8quuk07lnys2myvw", 10000, 3),
				genInput("tb1q60ccgeenqeu6ravga6s9e07pgd5n8t5c72vlhv", 10000, 4),
				// unused/not needed
				genInput("tb1qhj56dyrrjceh484szyjzec4snz56kwx4epd04r", 10000, 5),
			},
			totalUtxoSpend: 3 + 4,
			tos: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(14000),
				},
				{
					To:    xc.Address("tb1q74ddh4gvqqy0n23nmt4xtx5wsdyh5vt0gmt2m8"),
					Value: xc.NewAmountBlockchainFromUint64(100),
				},
			},
			expectedRecipients: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(14000),
				},
				{
					To:    xc.Address("tb1q74ddh4gvqqy0n23nmt4xtx5wsdyh5vt0gmt2m8"),
					Value: xc.NewAmountBlockchainFromUint64(100),
				},
				{
					// change
					To:    xc.Address("tb1q60ccgeenqeu6ravga6s9e07pgd5n8t5c72vlhv"),
					Value: xc.NewAmountBlockchainFromUint64(5900),
				},
			},
		},
		{
			name: "input contains unrelated address should be ignored",
			froms: []string{
				"tb1qlcpaaqyfqqraajlp7s4j4h8quuk07lnys2myvw",
				"tb1q60ccgeenqeu6ravga6s9e07pgd5n8t5c72vlhv",
				"tb1qhj56dyrrjceh484szyjzec4snz56kwx4epd04r",
			},
			fromInputs: []tx_input.TxInput{
				genInput("tb1qnavt59qjevnx5glhefm8nrlyq64cp34q3xrk7q", 10000000, 20), // NOT USED (not in trusted from set)
				genInput("tb1q60ccgeenqeu6ravga6s9e07pgd5n8t5c72vlhv", 10000, 4),
				// unused/not needed
				genInput("tb1qhj56dyrrjceh484szyjzec4snz56kwx4epd04r", 10000, 5),
			},
			totalUtxoSpend: 4 + 5,
			tos: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(15000),
				},
				{
					To:    xc.Address("tb1q74ddh4gvqqy0n23nmt4xtx5wsdyh5vt0gmt2m8"),
					Value: xc.NewAmountBlockchainFromUint64(100),
				},
			},
			expectedRecipients: []tx.Recipient{
				{
					To:    xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0"),
					Value: xc.NewAmountBlockchainFromUint64(15000),
				},
				{
					To:    xc.Address("tb1q74ddh4gvqqy0n23nmt4xtx5wsdyh5vt0gmt2m8"),
					Value: xc.NewAmountBlockchainFromUint64(100),
				},
				{
					// change
					To:    xc.Address("tb1qhj56dyrrjceh484szyjzec4snz56kwx4epd04r"),
					Value: xc.NewAmountBlockchainFromUint64(4900),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spenders := []*xcbuilder.Sender{}
			for i, from := range tc.fromInputs {
				if len(tc.froms) > 0 {
					spenders = append(spenders, buildertest.MustNewSender(xc.Address(tc.froms[i]), []byte{}))
				} else {
					spenders = append(spenders, buildertest.MustNewSender(from.Address, []byte{}))
				}
			}
			receivers := []*xcbuilder.Receiver{}
			for _, to := range tc.tos {
				receivers = append(receivers, buildertest.MustNewReceiver(to.To, to.Value))
			}
			args, err := xcbuilder.NewMultiTransferArgs(xc.BTC, spenders, receivers)
			require.NoError(err)
			tf, err := txBuilder.MultiTransfer(*args, &tx_input.MultiTransferInput{
				Inputs: tc.fromInputs,
				// exclude gas fees
				GasPricePerByte: xc.NewAmountBlockchainFromUint64(0),
				EstimatedSize:   0,
			})
			require.NoError(err)
			require.NotNil(tf)
			btcTx := tf.(*tx.Tx)
			require.Equal(tc.expectedRecipients, btcTx.Recipients)
			require.Equal(tc.totalUtxoSpend, len(btcTx.MsgTx.TxIn))

			// should be only a single change output
			sumRecipients := int64(0)
			change := btcTx.Recipients[len(btcTx.Recipients)-1].Value.Int().Int64()
			for _, recipient := range btcTx.Recipients {
				sumRecipients += recipient.Value.Int().Int64()
			}
			totalTransferAmount := int64(0)
			for _, recv := range receivers {
				totalTransferAmount += recv.GetAmount().Int().Int64()
			}
			// total transfer amount == sum of recipients - change amount
			require.Equal(totalTransferAmount, sumRecipients-change)
		})
	}

}
