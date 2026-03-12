package tx_input_test

import (
	"encoding/json"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInputPreparedTransactionJSONRoundTrip(t *testing.T) {
	t.Parallel()

	input := tx_input.NewTxInput()
	input.LedgerEnd = 123
	input.PreparedTransaction = &interactive.PreparedTransaction{
		Metadata: &interactive.Metadata{
			InputContracts: []*interactive.Metadata_InputContract{
				{
					Contract:  &interactive.Metadata_InputContract_V1{V1: &v1.Create{}},
					CreatedAt: 1,
					EventBlob: []byte{1, 2, 3},
				},
			},
		},
	}
	input.HashingSchemeVersion = interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2
	input.SubmissionId = "submission-id"
	input.DeduplicationWindow = 90 * time.Second
	input.Decimals = 10
	input.ContractAddress = xc.ContractAddress(xc.CANTON)

	raw, err := json.Marshal(input)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"v1"`)
	require.NotContains(t, string(raw), `"Contract"`)

	encoded, err := drivers.MarshalTxInput(input)
	require.NoError(t, err)

	decodedI, err := drivers.UnmarshalTxInput(encoded)
	require.NoError(t, err)
	decoded, ok := decodedI.(*tx_input.TxInput)
	require.True(t, ok)
	require.Equal(t, input.LedgerEnd, decoded.LedgerEnd)
	require.Equal(t, input.SubmissionId, decoded.SubmissionId)
	require.Equal(t, input.DeduplicationWindow, decoded.DeduplicationWindow)
	require.Equal(t, input.Decimals, decoded.Decimals)
	require.Equal(t, input.ContractAddress, decoded.ContractAddress)
	require.NotNil(t, decoded.PreparedTransaction)
	require.Len(t, decoded.PreparedTransaction.GetMetadata().GetInputContracts(), 1)
	require.NotNil(t, decoded.PreparedTransaction.GetMetadata().GetInputContracts()[0].GetV1())
}

func TestTxInputConflicts(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name string

		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}

	vectors := []testcase{
		{
			name:            "same submission id",
			newInput:        cantonTxInput("submission-id", nil, nil, []string{"consumed-cid"}),
			oldInput:        cantonTxInput("submission-id", nil, nil, []string{"other-consumed-cid"}),
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			name:            "overlapping consumed contract ids",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid"}),
			oldInput:        cantonTxInput("submission-b", nil, nil, []string{"consumed-cid"}),
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			name:            "disjoint consumed contract ids",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid-a"}),
			oldInput:        cantonTxInput("submission-b", nil, nil, []string{"consumed-cid-b"}),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:            "shared metadata and non-consuming contracts",
			newInput:        cantonTxInput("submission-a", []string{"metadata-cid"}, []string{"non-consuming-cid"}, []string{"consumed-cid-a"}),
			oldInput:        cantonTxInput("submission-b", []string{"metadata-cid"}, []string{"non-consuming-cid"}, []string{"consumed-cid-b"}),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:            "known empty consumed contract ids",
			newInput:        cantonTxInput("submission-a", nil, nil, nil),
			oldInput:        cantonTxInput("submission-b", []string{"input-cid-b"}, nil, nil),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:            "unknown transfer contract ids",
			newInput:        cantonTxInputWithoutPrepared("submission-a"),
			oldInput:        cantonTxInput("submission-b", nil, nil, []string{"consumed-cid-b"}),
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			name:            "different tx input type",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid"}),
			oldInput:        nil,
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			name:            "transfer and create account overlap consumed contract ids",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid"}),
			oldInput:        cantonCreateAccountInput(t, "party-a", tx_input.CreateAccountStageAccept, "submission-b", []string{"consumed-cid"}),
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			name:            "transfer and create account disjoint consumed contract ids",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid-a"}),
			oldInput:        cantonCreateAccountInput(t, "party-a", tx_input.CreateAccountStageAccept, "submission-b", []string{"consumed-cid-b"}),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:            "transfer and allocate create account",
			newInput:        cantonTxInput("submission-a", nil, nil, []string{"consumed-cid-a"}),
			oldInput:        cantonCreateAccountInput(t, "party-a", tx_input.CreateAccountStageAllocate, "", nil),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:            "same create account party and stage",
			newInput:        cantonCreateAccountInput(t, "party-a", tx_input.CreateAccountStageAllocate, "", nil),
			oldInput:        cantonCreateAccountInput(t, "party-a", tx_input.CreateAccountStageAllocate, "", nil),
			independent:     false,
			doubleSpendSafe: true,
		},
	}

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			t.Logf("expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))

			require.Equal(t, v.independent, v.newInput.IndependentOf(v.oldInput), "IndependentOf")
			require.Equal(t, v.doubleSpendSafe, v.newInput.SafeFromDoubleSend(v.oldInput), "SafeFromDoubleSend")
		})
	}
}

func TestTxInputConflictContractIDs(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name        string
		input       *TxInput
		contractIDs []string
	}

	vectors := []testcase{
		{
			name:        "consuming contract ids",
			input:       cantonTxInput("submission-id", []string{"metadata-cid"}, []string{"non-consuming-cid"}, []string{"consumed-cid"}),
			contractIDs: []string{"consumed-cid"},
		},
		{
			name:        "deduplicates contract ids",
			input:       cantonTxInput("submission-id", nil, nil, []string{"consumed-cid", "consumed-cid"}),
			contractIDs: []string{"consumed-cid"},
		},
		{
			name:        "none",
			input:       cantonTxInput("submission-id", []string{"metadata-cid"}, []string{"non-consuming-cid"}, nil),
			contractIDs: []string{},
		},
	}

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, v.contractIDs, v.input.ConflictContractIDs())
		})
	}
}

func cantonTxInput(submissionID string, inputContractIDs []string, nonConsumingContractIDs []string, consumingContractIDs []string) *TxInput {
	input := tx_input.NewTxInput()
	input.SubmissionId = submissionID

	inputContracts := make([]*interactive.Metadata_InputContract, 0, len(inputContractIDs))
	for _, contractID := range inputContractIDs {
		inputContracts = append(inputContracts, &interactive.Metadata_InputContract{
			Contract: &interactive.Metadata_InputContract_V1{
				V1: &v1.Create{ContractId: contractID},
			},
		})
	}

	nodes := []*interactive.DamlTransaction_Node{}
	for _, contractID := range nonConsumingContractIDs {
		nodes = append(nodes, &interactive.DamlTransaction_Node{
			NodeId: "non-consuming-exercise-node-" + contractID,
			VersionedNode: &interactive.DamlTransaction_Node_V1{
				V1: &v1.Node{
					NodeType: &v1.Node_Exercise{
						Exercise: &v1.Exercise{
							ContractId: contractID,
							ChoiceId:   "NonConsumingChoice",
							Consuming:  false,
						},
					},
				},
			},
		})
	}
	for _, contractID := range consumingContractIDs {
		nodes = append(nodes, &interactive.DamlTransaction_Node{
			NodeId: "consuming-exercise-node-" + contractID,
			VersionedNode: &interactive.DamlTransaction_Node_V1{
				V1: &v1.Node{
					NodeType: &v1.Node_Exercise{
						Exercise: &v1.Exercise{
							ContractId: contractID,
							ChoiceId:   "Archive",
							Consuming:  true,
						},
					},
				},
			},
		})
	}

	input.PreparedTransaction = &interactive.PreparedTransaction{
		Metadata: &interactive.Metadata{
			InputContracts: inputContracts,
		},
		Transaction: &interactive.DamlTransaction{
			Nodes: nodes,
		},
	}

	return input
}

func cantonTxInputWithoutPrepared(submissionID string) *TxInput {
	input := tx_input.NewTxInput()
	input.SubmissionId = submissionID
	return input
}

func cantonCreateAccountInput(t *testing.T, partyID string, stage string, submissionID string, consumingContractIDs []string) *tx_input.CreateAccountInput {
	t.Helper()

	input := &tx_input.CreateAccountInput{
		Stage:                     stage,
		PartyID:                   partyID,
		SetupProposalSubmissionID: submissionID,
	}
	if len(consumingContractIDs) == 0 {
		return input
	}

	input.SetupProposalPreparedTransaction = cantonTxInput(submissionID, nil, nil, consumingContractIDs).PreparedTransaction
	return input
}
