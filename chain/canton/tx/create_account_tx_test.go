package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/stretchr/testify/require"
)

const allocateCreateAccountPartyID = "e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8"

func TestCreateAccountTxRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *tx_input.CreateAccountInput
	}{
		{
			name: "allocate",
			input: &tx_input.CreateAccountInput{
				Stage:                tx_input.CreateAccountStageAllocate,
				PartyID:              allocateCreateAccountPartyID,
				TopologyTransactions: [][]byte{{0x01, 0x02}},
			},
		},
		{
			name:  "accept",
			input: loadLiveAcceptInput(t),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args, err := xcbuilder.NewCreateAccountArgs(xc.CANTON, xc.Address(tt.input.PartyID), []byte{0x01, 0x02})
			require.NoError(t, err)

			tx, err := cantontx.NewCreateAccountTx(args, tt.input)
			require.NoError(t, err)

			sighashes, err := tx.Sighashes()
			require.NoError(t, err)
			require.Len(t, sighashes, 1)
			if tt.input.Stage == tx_input.CreateAccountStageAllocate {
				expectedHash, err := cantontx.ComputeTopologyMultiHash(tt.input.TopologyTransactions)
				require.NoError(t, err)
				require.Equal(t, expectedHash, sighashes[0].Payload)
			} else {
				expectedHash, err := tx_input.ComputePreparedTransactionHash(tt.input.SetupProposalPreparedTransaction)
				require.NoError(t, err)
				require.Equal(t, expectedHash, sighashes[0].Payload)
			}

			unsigned, err := tx.Serialize()
			require.NoError(t, err)
			require.Len(t, unsigned, 8)

			metadataBz, ok, err := tx.GetMetadata()
			require.NoError(t, err)
			require.True(t, ok)
			metadata, err := cantontx.ParseMetadata(metadataBz)
			require.NoError(t, err)

			parsedUnsigned, err := cantontx.ParseCreateAccountTxWithMetadata(unsigned, metadata)
			require.NoError(t, err)
			require.Equal(t, tt.input.Stage, parsedUnsigned.Input.Stage)

			err = tx.SetSignatures(&xc.SignatureResponse{Signature: []byte{0xde, 0xad}})
			require.NoError(t, err)

			signed, err := tx.Serialize()
			require.NoError(t, err)

			parsedSigned, err := cantontx.ParseCreateAccountTxWithMetadata(signed, metadata)
			require.NoError(t, err)
			require.Equal(t, "dead", hex.EncodeToString(parsedSigned.Input.Signature))
			require.NotEmpty(t, tx.Hash())
		})
	}
}

func loadLiveAcceptInput(t *testing.T) *tx_input.CreateAccountInput {
	t.Helper()

	return &tx_input.CreateAccountInput{
		Stage:                            tx_input.CreateAccountStageAccept,
		PartyID:                          "0fd91a6a14378001c71f0690e05893727031531c826137a2c20be281b1588adf::12205e77deff729e037e823004308b9f26babea8aa9c0ce0e6f38362b715fda47c06",
		SetupProposalPreparedTransaction: hashTestPreparedTransaction("1", hashTestExerciseNode("ExternalPartySetupProposal_Accept", hashTestEmptyRecord())),
		SetupProposalSubmissionID:        "33436f5a-66fd-fcce-4689-3e8a791d1c5c",
	}
}
