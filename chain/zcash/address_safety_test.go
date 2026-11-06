package zcash_test

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/zcash"
	zaddress "github.com/cordialsys/crosschain/chain/zcash/address"
	"github.com/stretchr/testify/require"
)

func TestNU7RejectsUnsupportedDestinationsBeforeSigning(t *testing.T) {
	cfg := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")
	input, from, _ := nu7Input(t, cfg)
	txb, err := zcash.NewTxBuilder(cfg.Base())
	require.NoError(t, err)
	// A checksummed Sprout encoding must not be reinterpreted as transparent.
	sprout := base58.CheckEncode(append([]byte{0x9a}, bytes.Repeat([]byte{1}, 64)...), 0x16)
	// Sapling receiver and UA from the official unified_address.json vectors.
	saplingBytes := mustDecodeHex(t, "d8ef8293d26de832e7193f296ba1922d90f122c6135bc231eebd91efdb03b1a8606771cd4fd6480574d43e")
	words, err := bech32.ConvertBits(saplingBytes, 8, 5, true)
	require.NoError(t, err)
	sapling, err := bech32.Encode("zs", words)
	require.NoError(t, err)
	for name, destination := range map[string]string{
		"Sprout":                            sprout,
		"Sapling":                           sapling,
		"Unified with transparent receiver": "u1l8xunezsvhq8fgzfl7404m450nwnd76zshscn6nfys7vyz2ywyh4cc5daaq0c7q2su5lqfh23sp7fkf3kt27ve5948mzpfdvckzaect2jtte308mkwlycj2u0eac077wu70vqcetkxf",
		// Official ZIP 320 example; a valid TEX encoding, not a damaged t1 address.
		"TEX":                 "tex1s2rt77ggv6q989lr49rkgzmh5slsksa9khdgte",
		"wrong network P2PKH": "tmE1qEWnU9YiukiaYLTheczfRLHHSv5iGHP",
		"wrong network P2SH":  "t2ArHstteBF9AFLfGia4YjaYEzJiWrPX1fu",
		"bad checksum":        "t1UYsZVJkLPeMjxEtACvSxfWuNmddpWfxzt",
	} {
		t.Run(name, func(t *testing.T) {
			to := xc.Address(destination)
			require.Error(t, zcash.ValidateAddress(cfg.Base(), to))
			amount := xc.NewAmountBlockchainFromUint64(100000000)
			_, err := txb.NewNativeTransfer(from, to, amount, input)
			require.Error(t, err)
			args, err := builder.NewTransferArgs(cfg.Base(), from, to, amount)
			require.NoError(t, err)
			_, err = txb.Transfer(args, input)
			require.Error(t, err)
			multiArgs, err := builder.NewMultiTransferArgs(cfg.Base(),
				[]*builder.Sender{buildertest.MustNewSender(from, nil)},
				[]*builder.Receiver{buildertest.MustNewReceiver(to, amount)})
			require.NoError(t, err)
			multi := tx_input.NewMultiTransferInput()
			multi.Zcash, multi.Inputs = input.Zcash, []tx_input.TxInput{*input}
			_, err = txb.MultiTransfer(*multiArgs, multi)
			require.Error(t, err)
		})
	}
}

// This is an offline test with public, fixed test keys and synthetic UTXOs.
// Transparent outputs from both historical v4 and new v5 transactions must
// remain spendable by their existing keys in a v5 transaction.
func TestNU7SpendsLegacyAndNewTransparentOutputs(t *testing.T) {
	for _, parentVersion := range []string{"v4", "v5"} {
		t.Run(parentVersion, func(t *testing.T) {
			cfg := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")
			input, original, originalKey := nu7Input(t, cfg)
			otherKey, otherPubkey := btcec.PrivKeyFromBytes([]byte{2})
			ab, err := zaddress.NewAddressBuilder(cfg.Base())
			require.NoError(t, err)
			destination, err := ab.GetAddressFromPublicKey(otherPubkey.SerializeCompressed())
			require.NoError(t, err)
			parentBuilder, err := zcash.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			if parentVersion == "v4" {
				// The Flux route retains the same Sapling v4 wire format used
				// by historical Zcash transactions, with identical t1 scripts.
				parentBuilder, err = zcash.NewTxBuilder(xc.NewChainConfig(xc.FLUX).WithNet("mainnet").Base())
				require.NoError(t, err)
				input.ConsensusBranchId = 0xc8e71055 // historical NU6 Zcash branch
			}
			parent, err := parentBuilder.NewNativeTransfer(original, destination, xc.NewAmountBlockchainFromUint64(100000000), input)
			require.NoError(t, err)
			signTestTransfer(t, parent, originalKey)
			builtParent, err := parent.(*zcash.Tx).Build()
			require.NoError(t, err)
			if parentVersion == "v4" {
				require.EqualValues(t, 0x80000004, builtParent.Version)
			} else {
				require.EqualValues(t, 0x80000005, builtParent.Version)
			}
			output := builtParent.Outputs[0]
			require.True(t, txscript.IsPayToPubKeyHash(output.ScriptPubkey))
			require.Equal(t, btcutil.Hash160(otherPubkey.SerializeCompressed()), output.ScriptPubkey[3:23])
			parentHash := mustDecodeHex(t, string(parent.Hash()))
			for i, j := 0, len(parentHash)-1; i < j; i, j = i+1, j-1 {
				parentHash[i], parentHash[j] = parentHash[j], parentHash[i]
			}
			spend := tx_input.NewTxInput()
			spend.Zcash = (zcash.Client{Chain: cfg}).GetZcashInput(3)
			spend.UnspentOutputs = []tx_input.Output{{
				Outpoint:     tx_input.Outpoint{Hash: parentHash, Index: 0},
				Value:        xc.NewAmountBlockchainFromUint64(uint64(output.Amount)),
				PubKeyScript: output.ScriptPubkey, Address: destination,
			}}
			txb, err := zcash.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			returned, err := txb.NewNativeTransfer(destination, original, xc.NewAmountBlockchainFromUint64(90000000), spend)
			require.NoError(t, err)
			signTestTransfer(t, returned, otherKey)
			builtReturn, err := returned.(*zcash.Tx).Build()
			require.NoError(t, err)
			require.EqualValues(t, 0x80000005, builtReturn.Version)
			require.Equal(t, zcash.ConsensusBranchIDNU7, builtReturn.ConsensusBranchId)
			require.Equal(t, btcutil.Hash160(originalKey.PubKey().SerializeCompressed()), builtReturn.Outputs[0].ScriptPubkey[3:23])
			// Change stays locked to the second wallet's original public key.
			require.Equal(t, btcutil.Hash160(otherPubkey.SerializeCompressed()), builtReturn.Outputs[1].ScriptPubkey[3:23])
		})
	}
}

func signTestTransfer(t *testing.T, tx xc.Tx, key *btcec.PrivateKey) {
	t.Helper()
	requests, err := tx.Sighashes()
	require.NoError(t, err)
	responses := make([]*xc.SignatureResponse, len(requests))
	for i, request := range requests {
		responses[i] = &xc.SignatureResponse{Signature: ecdsa.SignCompact(key, request.Payload, true)[1:], PublicKey: key.PubKey().SerializeCompressed()}
	}
	require.NoError(t, tx.SetSignatures(responses...))
	_, err = tx.Serialize()
	require.NoError(t, err)
}
