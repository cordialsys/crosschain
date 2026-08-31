package address_test

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/stretchr/testify/require"

	xc "github.com/cordialsys/crosschain"
	bitcoinparams "github.com/cordialsys/crosschain/chain/bitcoin/params"
	"github.com/cordialsys/crosschain/chain/zcash/address"
)

func TestZcashAddressDecoderScripts(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		encoded     xc.Address
		addressType address.TransparentAddressType
		script      string
	}{
		{
			name:        "mainnet p2pkh",
			network:     "mainnet",
			encoded:     "t1NB5ugJ4ktDQcUP6fjPumKzfjJCdMmkM7v",
			addressType: address.TransparentAddressPubKeyHash,
			script:      "76a9142f2eddc4f91361f20797e00734662932fb5161c588ac",
		},
		{
			name:        "mainnet p2sh regression",
			network:     "mainnet",
			encoded:     "t3Ns6qDnWJnXnhe5Xnq4WBxMbspVLtQpMtf",
			addressType: address.TransparentAddressScriptHash,
			script:      "a9142f2eddc4f91361f20797e00734662932fb5161c587",
		},
		{
			name:        "testnet p2pkh",
			network:     "testnet",
			encoded:     "tmE1qEWnU9YiukiaYLTheczfRLHHSv5iGHP",
			addressType: address.TransparentAddressPubKeyHash,
			script:      "76a9142f2eddc4f91361f20797e00734662932fb5161c588ac",
		},
		{
			name:        "testnet p2sh",
			network:     "testnet",
			encoded:     "t2ArHstteBF9AFLfGia4YjaYEzJiWrPX1fu",
			addressType: address.TransparentAddressScriptHash,
			script:      "a9142f2eddc4f91361f20797e00734662932fb5161c587",
		},
	}

	decoder := address.NewAddressDecoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := xc.NewChainConfig(xc.ZEC).WithNet(tt.network)
			params, err := bitcoinparams.GetParams(cfg.Base())
			require.NoError(t, err)

			decoded, err := decoder.Decode(tt.encoded, &params)
			require.NoError(t, err)
			transparent, ok := decoded.(*address.TransparentAddress)
			require.True(t, ok)
			require.Equal(t, tt.addressType, transparent.Type)
			require.Equal(t, string(tt.encoded), transparent.EncodeAddress())
			require.True(t, transparent.IsForNet(&params))

			script, err := decoder.PayToAddrScript(transparent)
			require.NoError(t, err)
			require.Equal(t, tt.script, hex.EncodeToString(script))
		})
	}
}

func TestZcashAddressDecoderRejectsWrongNetworkAndUnknownPrefix(t *testing.T) {
	decoder := address.NewAddressDecoder()
	testnet := xc.NewChainConfig(xc.ZEC).WithNet("testnet")
	testnetParams, err := bitcoinparams.GetParams(testnet.Base())
	require.NoError(t, err)

	_, err = decoder.Decode("t3Ns6qDnWJnXnhe5Xnq4WBxMbspVLtQpMtf", &testnetParams)
	require.ErrorContains(t, err, "unsupported zcash address prefix")

	hash, err := hex.DecodeString("2f2eddc4f91361f20797e00734662932fb5161c5")
	require.NoError(t, err)
	unknownPrefixAddress := base58.CheckEncode(append([]byte{0xB9}, hash...), 0x1C)
	mainnet := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")
	mainnetParams, err := bitcoinparams.GetParams(mainnet.Base())
	require.NoError(t, err)

	_, err = decoder.Decode(xc.Address(unknownPrefixAddress), &mainnetParams)
	require.ErrorContains(t, err, "unsupported zcash address prefix")
}
