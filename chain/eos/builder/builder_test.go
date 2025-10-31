package builder_test

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/eos/builder"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	"github.com/cordialsys/crosschain/chain/eos/tx"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func randomSig() []byte {
	sig := make([]byte, 65)
	_, _ = rand.Read(sig)
	return sig
}

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(xc.NewChainConfig("EOS").Base())
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestTransferSignCompletes(t *testing.T) {
	chainCfg := xc.NewChainConfig("EOS").WithDecimals(4)
	builder1, err := builder.NewTxBuilder(chainCfg.Base())
	require.NoError(t, err)

	type testcase struct {
		args              buildertest.TransferArgs
		input             *TxInput
		expectedActions   int
		expectedSighashes int
		ramBuyer          string
	}

	testcases := []testcase{
		{
			args: buildertest.MustNewTransferArgs(
				chainCfg.Base(),
				xc.Address("from"),
				xc.Address("to"),
				xc.NewAmountBlockchainFromUint64(1),
			),
			input: &TxInput{
				Timestamp:    time.Now().Unix(),
				ChainID:      []byte{1, 2, 3, 4, 6, 7, 8},
				HeadBlockID:  make([]byte, 32),
				FromAccount:  "aaaaaaaaaaa1",
				AvailableRam: 2000,
				TargetRam:    2000,
			},
			expectedActions:   1,
			expectedSighashes: 1,
		},
		{
			args: buildertest.MustNewTransferArgs(
				chainCfg.Base(),
				xc.Address("from"),
				xc.Address("to"),
				xc.NewAmountBlockchainFromUint64(10000),
			),
			input: &TxInput{
				Timestamp:    time.Now().Unix(),
				ChainID:      []byte{1, 2, 3, 4, 6, 7, 8},
				HeadBlockID:  make([]byte, 32),
				FromAccount:  "aaaaaaaaaaa1",
				AvailableRam: 100,
				TargetRam:    2000,
			},
			// should be extra action to buy ram
			expectedActions:   2,
			ramBuyer:          "aaaaaaaaaaa1",
			expectedSighashes: 1,
		},
		{
			args: buildertest.MustNewTransferArgs(
				chainCfg.Base(),
				xc.Address("from"),
				xc.Address("to"),
				xc.NewAmountBlockchainFromUint64(10000),
				buildertest.OptionFeePayer(xc.Address("co-signer"), []byte{}),
			),
			input: &TxInput{
				Timestamp:       time.Now().Unix(),
				ChainID:         []byte{1, 2, 3, 4, 6, 7, 8},
				HeadBlockID:     make([]byte, 32),
				FromAccount:     "aaaaaaaaaaa1",
				FeePayerAccount: "aaaaaaaaaaa3",
				AvailableRam:    1900,
				TargetRam:       2000,
			},
			// no extra action to buy ram since it should be within tolerance/threshold
			expectedActions: 1,
			// should be extra sig for co-signer
			expectedSighashes: 2,
		},
		{
			args: buildertest.MustNewTransferArgs(
				chainCfg.Base(),
				xc.Address("from"),
				xc.Address("to"),
				xc.NewAmountBlockchainFromUint64(10000),
				buildertest.OptionFeePayer(xc.Address("co-signer"), []byte{}),
			),
			input: &TxInput{
				Timestamp:       time.Now().Unix(),
				ChainID:         []byte{1, 2, 3, 4, 6, 7, 8},
				HeadBlockID:     make([]byte, 32),
				FromAccount:     "aaaaaaaaaaa1",
				FeePayerAccount: "aaaaaaaaaaa3",
				AvailableRam:    500,
				TargetRam:       2000,
			},
			expectedActions: 2,
			// should be extra sig for co-signer
			expectedSighashes: 2,
			// fee-payer should be the ram buyer
			ramBuyer: "aaaaaaaaaaa3",
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			tf, err := builder1.Transfer(tc.args, tc.input)
			require.NoError(t, err)

			eosTx := tf.(*tx.Tx)
			builtTx, err := eosTx.BuildTx()
			require.NoError(t, err)
			require.Equal(t, tc.expectedActions, len(builtTx.Actions))

			if tc.ramBuyer != "" {
				data := builtTx.Actions[0].Data.(action.BuyRamBytes)
				require.Equal(t, tc.ramBuyer, data.Payer.String())
			} else {
				_, ok := builtTx.Actions[0].Data.(action.BuyRamBytes)
				require.False(t, ok)
			}

			sigHashes, err := tf.Sighashes()
			require.NoError(t, err)
			require.Equal(t, tc.expectedSighashes, len(sigHashes))

			// run this a few times since it's a bit probabilistic
			for range 10 {
				signatures := []*xc.SignatureResponse{}
				for _, sigHash := range sigHashes {
					signatures = append(signatures, &xc.SignatureResponse{
						Address:   sigHash.Signer,
						Signature: randomSig(),
					})
				}
				err = tf.SetSignatures(signatures...)
				require.NoError(t, err)

				const max = 250

				for i := range max {
					additionalTf, ok := tf.(xc.TxAdditionalSighashes)
					require.True(t, ok, "should implement TxAdditionalSighashes")

					additionalSighashes, err := additionalTf.AdditionalSighashes()
					require.NoError(t, err)
					if len(additionalSighashes) == 0 {
						break
					}

					for _, sigHash := range additionalSighashes {
						signatures = append(signatures, &xc.SignatureResponse{
							Address:   sigHash.Signer,
							Signature: randomSig(),
						})
					}
					err = tf.SetSignatures(signatures...)
					require.NoError(t, err)

					if i >= (max - 1) {
						require.Fail(t, "transaction is requesting signatures endlessly")
					}
				}
			}
		})
	}
}
