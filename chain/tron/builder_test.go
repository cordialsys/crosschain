package tron_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/tron"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func randomSig() []byte {
	sig := make([]byte, 65)
	_, _ = rand.Read(sig)
	return sig
}

func TestNewTxBuilder(t *testing.T) {
	type testcase struct {
		input          *tron.TxInput
		args           buildertest.TransferArgs
		expectedSigHex string
	}
	chainCfg := xc.NewChainConfig(xc.TRX).WithDecimals(6).Base()

	testcases := []testcase{
		{
			// native transfer
			input: &tron.TxInput{
				TxInputEnvelope: tron.NewTxInput().TxInputEnvelope,
				RefBlockBytes:   testutil.FromHex("5273"),
				RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
				Expiration:      200,
				Timestamp:       100,
				MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
			},
			args: buildertest.MustNewTransferArgs(
				chainCfg,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				xc.Address("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
				xc.NewAmountBlockchainFromUint64(10000),
			),
			expectedSigHex: "bffb93894087cb83be9a9546afb83da420cb67fceefb99a28316532ffa4c9ede",
		},
		{
			// token transfer
			input: &tron.TxInput{
				TxInputEnvelope: tron.NewTxInput().TxInputEnvelope,
				RefBlockBytes:   testutil.FromHex("5273"),
				RefBlockHash:    testutil.FromHex("40c45983779ab5f8"),
				Expiration:      200,
				Timestamp:       100,
				MaxFee:          xc.NewAmountBlockchainFromUint64(1000000),
			},
			args: buildertest.MustNewTransferArgs(
				chainCfg,
				xc.Address("TFmgAF3HfTJZk2aHkvSu8FDtVArbqp4XE5"),
				xc.Address("TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8"),
				xc.NewAmountBlockchainFromUint64(10000),
				buildertest.OptionContractAddress("TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"),
				buildertest.OptionContractDecimals(6),
			),
			expectedSigHex: "a6d38223922a3210653eb97b71bd98f47eebecbbc5fc5f3d60dbadc69c68ee68",
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			txBuilder, err := tron.NewTxBuilder(chainCfg)
			require.NoError(t, err)
			require.NotNil(t, txBuilder)

			txI, err := txBuilder.Transfer(tc.args, tc.input)
			require.NoError(t, err)
			require.NotNil(t, txI)

			hashes, err := txI.Sighashes()
			require.NoError(t, err)
			require.Len(t, hashes, 1)

			require.Equal(t, tc.expectedSigHex, hex.EncodeToString(hashes[0].Payload))

			err = txI.SetSignatures(&xc.SignatureResponse{
				Signature: randomSig(),
			})
			require.NoError(t, err)

			bz, err := txI.Serialize()
			require.NoError(t, err)
			require.True(t, len(bz) > 0)
		})
	}
}
