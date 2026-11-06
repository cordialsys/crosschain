package zcash_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/zcash"
	zaddress "github.com/cordialsys/crosschain/chain/zcash/address"
	"github.com/cordialsys/crosschain/factory/defaults/chains"
	"github.com/stretchr/testify/require"
)

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

func nu7Input(t *testing.T, cfg *xc.ChainConfig) (*tx_input.TxInput, xc.Address, *btcec.PrivateKey) {
	t.Helper()
	key, pubkey := btcec.PrivKeyFromBytes([]byte{1})
	ab, err := zaddress.NewAddressBuilder(cfg.Base())
	require.NoError(t, err)
	from, err := ab.GetAddressFromPublicKey(pubkey.SerializeCompressed())
	require.NoError(t, err)
	script, err := txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).
		AddData(btcutil.Hash160(pubkey.SerializeCompressed())).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).Script()
	require.NoError(t, err)
	input := tx_input.NewTxInput()
	input.Address = from
	input.Zcash = (zcash.Client{Chain: cfg}).GetZcashInput(4)
	input.UnspentOutputs = []tx_input.Output{
		{Outpoint: tx_input.Outpoint{Hash: bytes.Repeat([]byte{1}, 32), Index: 2}, Value: xc.NewAmountBlockchainFromUint64(150000000), PubKeyScript: script, Address: from},
		{Outpoint: tx_input.Outpoint{Hash: bytes.Repeat([]byte{2}, 32), Index: 3}, Value: xc.NewAmountBlockchainFromUint64(250000000), PubKeyScript: script, Address: from},
	}
	return input, from, key
}

func TestNU7TransferSigning(t *testing.T) {
	for _, network := range []string{"mainnet", "testnet"} {
		for _, method := range []string{"transfer", "native", "multi"} {
			t.Run(network+"/"+method, func(t *testing.T) {
				cfg := xc.NewChainConfig(xc.ZEC).WithNet(network)
				input, from, key := nu7Input(t, cfg)
				// Offline input envelopes preserve the consensus branch and prevouts.
				encoded, err := json.Marshal(input)
				require.NoError(t, err)
				var restored tx_input.TxInput
				require.NoError(t, json.Unmarshal(encoded, &restored))
				require.Equal(t, zcash.ConsensusBranchIDNU7, restored.ConsensusBranchId)
				txb, err := zcash.NewTxBuilder(cfg.Base())
				require.NoError(t, err)
				amount := xc.NewAmountBlockchainFromUint64(100000000)
				var tx xc.Tx
				switch method {
				case "transfer":
					args, err := builder.NewTransferArgs(cfg.Base(), from, from, amount)
					require.NoError(t, err)
					tx, err = txb.Transfer(args, &restored)
					require.NoError(t, err)
				case "native":
					tx, err = txb.NewNativeTransfer(from, from, amount, &restored)
					require.NoError(t, err)
				case "multi":
					args, err := builder.NewMultiTransferArgs(cfg.Base(),
						[]*builder.Sender{buildertest.MustNewSender(from, nil)},
						[]*builder.Receiver{buildertest.MustNewReceiver(from, amount)})
					require.NoError(t, err)
					multi := tx_input.NewMultiTransferInput()
					multi.Zcash, multi.Inputs = restored.Zcash, []tx_input.TxInput{restored}
					tx, err = txb.MultiTransfer(*args, multi)
					require.NoError(t, err)
				}
				requests, err := tx.Sighashes()
				require.NoError(t, err)
				require.Len(t, requests, 2)
				require.NotEqual(t, requests[0].Payload, requests[1].Payload)
				unsignedID := tx.Hash()
				require.NotEmpty(t, unsignedID)
				responses := make([]*xc.SignatureResponse, len(requests))
				for i, request := range requests {
					compact := ecdsa.SignCompact(key, request.Payload, true)
					responses[i] = &xc.SignatureResponse{Signature: compact[1:], PublicKey: key.PubKey().SerializeCompressed()}
				}
				require.NoError(t, tx.SetSignatures(responses...))
				require.Equal(t, unsignedID, tx.Hash())
				raw, err := tx.Serialize()
				require.NoError(t, err)
				require.Equal(t, uint32(0x80000005), binary.LittleEndian.Uint32(raw[:4]))
				require.Equal(t, uint32(0x26a7270a), binary.LittleEndian.Uint32(raw[4:8]))
				require.Equal(t, zcash.ConsensusBranchIDNU7, binary.LittleEndian.Uint32(raw[8:12]))
				require.Equal(t, []byte{0, 0, 0}, raw[len(raw)-3:])
				built, err := tx.(*zcash.Tx).Build()
				require.NoError(t, err)
				for i, in := range built.Inputs {
					pushes, err := txscript.PushedData(in.SignatureScript)
					require.NoError(t, err)
					require.Len(t, pushes, 2)
					sigWithType := pushes[0]
					require.Equal(t, byte(txscript.SigHashAll), sigWithType[len(sigWithType)-1])
					sig, err := ecdsa.ParseDERSignature(sigWithType[:len(sigWithType)-1])
					require.NoError(t, err)
					require.True(t, sig.Verify(requests[i].Payload, key.PubKey()))
				}
				// Mutating caller-owned signature buffers must not change the transaction.
				responses[0].Signature[0] ^= 1
				responses[0].PublicKey[0] ^= 1
				after, err := tx.Serialize()
				require.NoError(t, err)
				require.Equal(t, raw, after)
			})
		}
	}
}

func TestNU7BuilderRejectsInvalidInput(t *testing.T) {
	cfg := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")
	txb, err := zcash.NewTxBuilder(cfg.Base())
	require.NoError(t, err)
	for name, mutate := range map[string]func(*tx_input.TxInput){
		"missing branch": func(in *tx_input.TxInput) { in.ConsensusBranchId = 0 },
		"missing script": func(in *tx_input.TxInput) { in.UnspentOutputs[0].PubKeyScript = nil },
		"P2SH spend": func(in *tx_input.TxInput) {
			in.UnspentOutputs[0].PubKeyScript = mustDecodeHex(t, "a914010101010101010101010101010101010101010187")
		},
		"missing outpoint": func(in *tx_input.TxInput) { in.UnspentOutputs[0].Hash = nil },
		"overflow amount": func(in *tx_input.TxInput) {
			in.UnspentOutputs[0].Value = xc.NewAmountBlockchainFromUint64(^uint64(0))
		},
	} {
		t.Run(name, func(t *testing.T) {
			input, from, _ := nu7Input(t, cfg)
			mutate(input)
			_, err := txb.NewNativeTransfer(from, from, xc.NewAmountBlockchainFromUint64(100000000), input)
			require.Error(t, err)
		})
	}
}

func TestNU7BranchConfiguration(t *testing.T) {
	for _, network := range []string{"mainnet", "testnet"} {
		cfg := xc.NewChainConfig(xc.ZEC).WithNet(network)
		input := (zcash.Client{Chain: cfg}).GetZcashInput(2)
		require.Equal(t, zcash.ConsensusBranchIDNU7, input.ConsensusBranchId)
		cfg.ChainID = "0xc2d6d0b4"
		_, err := zcash.NewClient(cfg)
		require.NoError(t, err)
		input = (zcash.Client{Chain: cfg}).GetZcashInput(2)
		require.Equal(t, uint32(0xc2d6d0b4), input.ConsensusBranchId)
		for _, invalid := range []xc.StringOrInt{"0", "garbage", "-1", "0x100000000"} {
			cfg.ChainID = invalid
			_, err := zcash.NewClient(cfg)
			require.ErrorContains(t, err, "nonzero uint32")
		}
	}
	for _, cfg := range []*xc.ChainConfig{chains.Mainnet["zec"], chains.Testnet["zec"]} {
		require.NotNil(t, cfg)
		branch, ok := cfg.ChainID.AsInt()
		require.True(t, ok)
		require.EqualValues(t, zcash.ConsensusBranchIDNU7, branch)
	}
	flux := (zcash.Client{Chain: chains.Mainnet["flux"]}).GetZcashInput(2)
	require.Equal(t, uint32(0x76B809BB), flux.ConsensusBranchId)
}

type inputClient struct{ bitcoin.BtcClient }

func (inputClient) FetchTransferInput(context.Context, builder.TransferArgs) (xc.TxInput, error) {
	return tx_input.NewTxInput(), nil
}

func (inputClient) FetchMultiTransferInput(context.Context, builder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	return tx_input.NewMultiTransferInput(), nil
}

func TestNU7ClientEnrichesInputs(t *testing.T) {
	client := zcash.Client{Chain: xc.NewChainConfig(xc.ZEC).WithNet("mainnet"), BtcClient: inputClient{}}
	input, err := client.FetchTransferInput(context.Background(), builder.TransferArgs{})
	require.NoError(t, err)
	require.Equal(t, zcash.ConsensusBranchIDNU7, input.(*tx_input.TxInput).ConsensusBranchId)
	multi, err := client.FetchMultiTransferInput(context.Background(), builder.MultiTransferArgs{})
	require.NoError(t, err)
	require.Equal(t, zcash.ConsensusBranchIDNU7, multi.(*tx_input.MultiTransferInput).ConsensusBranchId)
}
