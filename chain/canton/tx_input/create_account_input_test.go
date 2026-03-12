package tx_input_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	"github.com/stretchr/testify/require"
)

func TestCreateAccountInputSetSignatures(t *testing.T) {
	t.Parallel()

	input := &tx_input.CreateAccountInput{}
	err := input.SetSignatures(&xc.SignatureResponse{Signature: []byte{0x01}})
	require.NoError(t, err)
	require.Equal(t, []byte{0x01}, input.Signature)

	err = input.SetSignatures()
	require.ErrorContains(t, err, "expected 1 signature")
}

func TestCreateAccountInputVerifySignaturePayloadsAllocateStage(t *testing.T) {
	t.Parallel()

	input := &tx_input.CreateAccountInput{
		Stage:                tx_input.CreateAccountStageAllocate,
		PartyID:              "e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8",
		TopologyTransactions: [][]byte{{0x01, 0x02}},
	}

	require.NoError(t, input.VerifySignaturePayloads())
}

func TestCreateAccountInputPreparedTransactionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := &tx_input.CreateAccountInput{
		Stage:   tx_input.CreateAccountStageAccept,
		PartyID: "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SetupProposalPreparedTransaction: &interactive.PreparedTransaction{
			Transaction: &interactive.DamlTransaction{
				Nodes: []*interactive.DamlTransaction_Node{
					{
						NodeId: "node-id",
						VersionedNode: &interactive.DamlTransaction_Node_V1{
							V1: &v1.Node{
								NodeType: &v1.Node_Exercise{
									Exercise: &v1.Exercise{ContractId: "contract-id", Consuming: true},
								},
							},
						},
					},
				},
			},
		},
		SetupProposalHashing:      interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
		SetupProposalSubmissionID: "submission-id",
	}

	raw, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"v1"`)
	require.NotContains(t, string(raw), `"Contract"`)

	var decoded tx_input.CreateAccountInput
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, input.Stage, decoded.Stage)
	require.Equal(t, input.PartyID, decoded.PartyID)
	require.Equal(t, input.SetupProposalHashing, decoded.SetupProposalHashing)
	require.Equal(t, input.SetupProposalSubmissionID, decoded.SetupProposalSubmissionID)
	require.NotNil(t, decoded.SetupProposalPreparedTransaction)
	require.Len(t, decoded.SetupProposalPreparedTransaction.GetTransaction().GetNodes(), 1)
}
