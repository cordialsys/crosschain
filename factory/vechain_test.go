package factory_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/vechain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/stretchr/testify/require"
)

func TestVeChainFactory(t *testing.T) {
	for _, f := range []*factory.Factory{factory.NewDefaultFactory(), factory.NewNotMainnetsFactory(nil)} {
		t.Run(string(f.Config.Network), func(t *testing.T) {
			cfg, ok := f.GetChain(xc.VET)
			require.True(t, ok)
			require.Equal(t, xc.DriverVeChain, cfg.Driver)
			client, err := drivers.NewClient(cfg, cfg.Driver)
			require.NoError(t, err)
			require.IsType(t, &vechain.Client{}, client)
			signer, err := f.NewSigner(cfg.Base(), "7582be841ca040aa940fff6c05773129e135623e41acce3e0b8ba520dc1ae26a")
			require.NoError(t, err)
			addressBuilder, err := f.NewAddressBuilder(cfg.Base())
			require.NoError(t, err)
			publicKey, err := signer.PublicKey()
			require.NoError(t, err)
			from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
			require.NoError(t, err)
			require.NoError(t, drivers.ValidateAddress(cfg.Base(), from))
			input := vechain.NewTxInput()
			if f.Config.Network == "testnet" {
				input.ChainTag = 0x27
			} else {
				input.ChainTag = 0x4a
			}
			input.FromAddress, input.BlockRef, input.Nonce = from, 0x0000000a851caf3c, 123
			input.GasLimit = 21000
			input.GasFeeCap = xc.NewAmountBlockchainFromUint64(100)
			serialized, err := f.MarshalTxInput(input)
			require.NoError(t, err)
			decoded, err := f.UnmarshalTxInput(serialized)
			require.NoError(t, err)
			require.IsType(t, &vechain.TxInput{}, decoded)
			require.Equal(t, input, decoded)
			args, err := builder.NewTransferArgs(cfg.Base(), from, "0x7567d83b7b8d80addcb281a71d54fc7b3364ffed", xc.NewAmountBlockchainFromUint64(42))
			require.NoError(t, err)
			txBuilder, err := f.NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			tx, err := txBuilder.Transfer(args, decoded)
			require.NoError(t, err)
			requests, err := tx.Sighashes()
			require.NoError(t, err)
			response, err := signer.Sign(requests[0])
			require.NoError(t, err)
			require.NoError(t, tx.SetSignatures(response))
			require.NotEmpty(t, tx.Hash())
			require.NoError(t, xc.CheckFeeLimit(input, cfg))
		})
	}
}
