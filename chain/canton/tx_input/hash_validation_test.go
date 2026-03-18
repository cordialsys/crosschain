package tx_input

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	v1 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive/transaction/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestValidatePreparedTransactionHash_Flows(t *testing.T) {
	t.Parallel()

	vectors := []struct {
		name       string
		preparedTx *interactive.PreparedTransaction
		verify     func(t *testing.T, preparedTx *interactive.PreparedTransaction, hash []byte)
	}{
		{
			name:       "transfer_preapproval_send",
			preparedTx: testPreparedTransaction("node-1", testExerciseNode("TransferPreapproval_Send", testAmountRecord("10.0"))),
			verify: func(t *testing.T, preparedTx *interactive.PreparedTransaction, hash []byte) {
				t.Helper()
				require.NoError(t, ValidatePreparedTransactionHash(preparedTx, hash))
			},
		},
		{
			name:       "transfer_offer_send",
			preparedTx: testPreparedTransaction("node-1", testCreateNode("TransferOffer", testTransferOfferArgument("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "10.0"))),
			verify: func(t *testing.T, preparedTx *interactive.PreparedTransaction, hash []byte) {
				t.Helper()
				require.NoError(t, ValidatePreparedTransactionHash(preparedTx, hash))
			},
		},
		{
			name:       "create_account_accept",
			preparedTx: testPreparedTransaction("node-1", testExerciseNode("ExternalPartySetupProposal_Accept", testEmptyRecord())),
			verify: func(t *testing.T, preparedTx *interactive.PreparedTransaction, hash []byte) {
				t.Helper()
				preparedBz, err := proto.Marshal(preparedTx)
				require.NoError(t, err)

				input := &CreateAccountInput{
					Stage:                            CreateAccountStageAccept,
					PartyID:                          "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SetupProposalPreparedTransaction: preparedBz,
					SetupProposalHash:                hash,
				}
				require.NoError(t, input.VerifySignaturePayloads())
			},
		},
		{
			name:       "complete_transfer_offer",
			preparedTx: testPreparedTransaction("node-1", testExerciseNode("AcceptedTransferOffer_Complete", testEmptyRecord())),
			verify: func(t *testing.T, preparedTx *interactive.PreparedTransaction, hash []byte) {
				t.Helper()
				require.NoError(t, ValidatePreparedTransactionHash(preparedTx, hash))
			},
		},
	}

	for _, vector := range vectors {
		vector := vector
		t.Run(vector.name, func(t *testing.T) {
			t.Parallel()

			hash := testPreparedTransactionHash(t, vector.preparedTx)
			vector.verify(t, vector.preparedTx, hash)

			wrongHash := append([]byte(nil), hash...)
			wrongHash[len(wrongHash)-1] ^= 0xff

			switch vector.name {
			case "create_account_accept":
				preparedBz, err := proto.Marshal(vector.preparedTx)
				require.NoError(t, err)
				input := &CreateAccountInput{
					Stage:                            CreateAccountStageAccept,
					PartyID:                          "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SetupProposalPreparedTransaction: preparedBz,
					SetupProposalHash:                wrongHash,
				}
				require.NoError(t, input.VerifySignaturePayloads())
			default:
				require.ErrorContains(t, ValidatePreparedTransactionHash(vector.preparedTx, wrongHash), "prepared transaction hash mismatch")
			}
		})
	}
}

func TestValidatePreparedTransactionHash_LiveCreateAccountAcceptMismatch(t *testing.T) {
	t.Parallel()

	input := mustLoadLiveCreateAccountAcceptInput(t)

	var preparedTx interactive.PreparedTransaction
	require.NoError(t, proto.Unmarshal(input.SetupProposalPreparedTransaction, &preparedTx))

	err := ValidatePreparedTransactionHash(&preparedTx, input.SetupProposalHash)
	require.ErrorContains(t, err, "prepared transaction hash mismatch")

	require.NoError(t, input.VerifySignaturePayloads())
}

func mustLoadLiveCreateAccountAcceptInput(t *testing.T) *CreateAccountInput {
	t.Helper()

	data, err := os.ReadFile("testdata/live_create_account_accept.json")
	require.NoError(t, err)

	var fixture struct {
		CreateAccountInput string `json:"create_account_input"`
	}
	require.NoError(t, json.Unmarshal(data, &fixture))

	encoded, err := hex.DecodeString(fixture.CreateAccountInput)
	require.NoError(t, err)

	input, err := ParseCreateAccountInput(encoded)
	require.NoError(t, err)
	return input
}

func testPreparedTransaction(nodeID string, node *v1.Node) *interactive.PreparedTransaction {
	return &interactive.PreparedTransaction{
		Transaction: &interactive.DamlTransaction{
			Version: "2",
			Roots:   []string{nodeID},
			Nodes: []*interactive.DamlTransaction_Node{
				{
					NodeId: nodeID,
					VersionedNode: &interactive.DamlTransaction_Node_V1{
						V1: node,
					},
				},
			},
		},
	}
}

func testPreparedTransactionHash(t *testing.T, preparedTx *interactive.PreparedTransaction) []byte {
	t.Helper()
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(preparedTx)
	require.NoError(t, err)

	sum := sha256.Sum256(data)
	return sum[:]
}

func testCreateNode(entity string, argument *v2.Value) *v1.Node {
	return &v1.Node{
		NodeType: &v1.Node_Create{
			Create: &v1.Create{
				TemplateId: &v2.Identifier{
					PackageId:  "pkg",
					ModuleName: "Splice.Wallet.TransferOffer",
					EntityName: entity,
				},
				Argument: argument,
			},
		},
	}
}

func testExerciseNode(choice string, chosenValue *v2.Value) *v1.Node {
	return &v1.Node{
		NodeType: &v1.Node_Exercise{
			Exercise: &v1.Exercise{
				TemplateId: &v2.Identifier{
					PackageId:  "pkg",
					ModuleName: "Splice.Wallet",
					EntityName: "Any",
				},
				ChoiceId:    choice,
				ChosenValue: chosenValue,
			},
		},
	}
}

func testTransferOfferArgument(receiver string, amount string) *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{
				Fields: []*v2.RecordField{
					{
						Label: "receiver",
						Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}},
					},
					{
						Label: "amount",
						Value: &v2.Value{
							Sum: &v2.Value_Record{
								Record: &v2.Record{
									Fields: []*v2.RecordField{
										{
											Label: "amount",
											Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func testAmountRecord(amount string) *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{
				Fields: []*v2.RecordField{
					{
						Label: "amount",
						Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}},
					},
				},
			},
		},
	}
}

func testEmptyRecord() *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{},
		},
	}
}
