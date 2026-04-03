package tx_test

import (
	"bytes"
	"strconv"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestComputePreparedTransactionHash(t *testing.T) {
	t.Parallel()

	preparedTx := hashTestPreparedTransaction("1", hashTestExerciseNode("TransferPreapproval_Send", hashTestAmountRecord("10.0")))

	hash1, err := tx_input.ComputePreparedTransactionHash(preparedTx)
	require.NoError(t, err)

	hash2, err := tx_input.ComputePreparedTransactionHash(preparedTx)
	require.NoError(t, err)

	require.Equal(t, hash1, hash2)
	require.NotEmpty(t, hash1)
}

func TestValidatePreparedTransactionHashFlows(t *testing.T) {
	t.Parallel()

	vectors := []struct {
		name       string
		preparedTx *interactive.PreparedTransaction
	}{
		{
			name:       "transfer_preapproval_send",
			preparedTx: hashTestPreparedTransaction("1", hashTestExerciseNode("TransferPreapproval_Send", hashTestAmountRecord("10.0"))),
		},
		{
			name:       "transfer_offer_send",
			preparedTx: hashTestPreparedTransaction("1", hashTestCreateNode("TransferOffer", hashTestTransferOfferArgument("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "10.0"))),
		},
		{
			name:       "create_account_accept",
			preparedTx: hashTestPreparedTransaction("1", hashTestExerciseNode("ExternalPartySetupProposal_Accept", hashTestEmptyRecord())),
		},
		{
			name:       "complete_transfer_offer",
			preparedTx: hashTestPreparedTransaction("1", hashTestExerciseNode("AcceptedTransferOffer_Complete", hashTestEmptyRecord())),
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			t.Parallel()

			hash := hashTestPreparedTransactionHash(t, vector.preparedTx)
			require.NoError(t, tx_input.ValidatePreparedTransactionHash(vector.preparedTx, hash))

			wrongHash := append([]byte(nil), hash...)
			wrongHash[len(wrongHash)-1] ^= 0xff
			require.ErrorContains(t, tx_input.ValidatePreparedTransactionHash(vector.preparedTx, wrongHash), "prepared transaction hash mismatch")
		})
	}
}

func TestCreateAccountAcceptSighashes_UsesPreparedTransactionHash(t *testing.T) {
	t.Parallel()

	input := &tx_input.CreateAccountInput{
		Stage:   tx_input.CreateAccountStageAccept,
		PartyID: "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SetupProposalAcceptInput: &tx_input.CreateAccountAcceptInput{
			TransactionVersion: "2",
			SubmitterActAs:     []string{"sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			SynchronizerID:     "sync-id",
			CommandID:          "command-id",
			SubmissionID:       "submission-id",
			Hashing:            interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
			TransactionUUID:    "transaction-uuid",
			PreparationTime:    1,
			Exercise: tx_input.CreateAccountAcceptExercise{
				Contract: tx_input.CreateAccountContractInfo{
					LfVersion:    "2",
					ContractID:   "001122",
					PackageName:  "splice-wallet",
					TemplateID:   &v2.Identifier{PackageId: "pkg", ModuleName: "Splice.Wallet", EntityName: "Any"},
					Signatories:  []string{"sig"},
					Stakeholders: []string{"stake"},
				},
				ActingParties: []string{"sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			},
			ProposalInputContract: tx_input.CreateAccountAcceptProposalContract{
				Contract: tx_input.CreateAccountContractInfo{
					LfVersion:    "2",
					ContractID:   "001122",
					PackageName:  "splice-wallet",
					TemplateID:   &v2.Identifier{PackageId: "pkg", ModuleName: "Splice.AmuletRules", EntityName: "ExternalPartySetupProposal"},
					Signatories:  []string{"sig"},
					Stakeholders: []string{"stake"},
				},
				CreatedAt:            1,
				CreatedEventBlob:     []byte{0x01},
				Validator:            "validator",
				User:                 "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				DSO:                  "dso",
				ProposalCreatedAt:    1,
				PreapprovalExpiresAt: 2,
			},
			ValidatorRight: tx_input.CreateAccountAcceptValidatorRight{
				Contract: tx_input.CreateAccountContractInfo{
					LfVersion:    "2",
					ContractID:   "001122aa",
					PackageName:  "splice-wallet",
					TemplateID:   &v2.Identifier{PackageId: "pkg", ModuleName: "Splice.Amulet", EntityName: "ValidatorRight"},
					Signatories:  []string{"sig"},
					Stakeholders: []string{"stake"},
				},
				DSO:       "dso",
				User:      "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Validator: "validator",
			},
			TransferPreapproval: tx_input.CreateAccountAcceptTransferPreapproval{
				Contract: tx_input.CreateAccountContractInfo{
					LfVersion:    "2",
					ContractID:   "001122bb",
					PackageName:  "splice-wallet",
					TemplateID:   &v2.Identifier{PackageId: "pkg", ModuleName: "Splice.AmuletRules", EntityName: "TransferPreapproval"},
					Signatories:  []string{"sig"},
					Stakeholders: []string{"stake"},
				},
				DSO:           "dso",
				Receiver:      "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Provider:      "validator",
				ValidFrom:     1,
				LastRenewedAt: 1,
				ExpiresAt:     2,
			},
			NodeSeeds: []tx_input.CreateAccountNodeSeed{
				{NodeID: 0, Seed: bytes.Repeat([]byte{0x11}, 32)},
				{NodeID: 1, Seed: bytes.Repeat([]byte{0x12}, 32)},
				{NodeID: 2, Seed: bytes.Repeat([]byte{0x13}, 32)},
			},
		},
	}

	args, err := xcbuilder.NewCreateAccountArgs(xc.CANTON, xc.Address(input.PartyID), []byte{0x01, 0x02})
	require.NoError(t, err)

	tx, err := cantontx.NewCreateAccountTx(args, input)
	require.NoError(t, err)

	sighashes, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, sighashes, 1)

	preparedTx, err := cantontx.BuildCreateAccountAcceptPreparedTransaction(input.SetupProposalAcceptInput)
	require.NoError(t, err)

	expectedHash, err := tx_input.ComputePreparedTransactionHash(preparedTx)
	require.NoError(t, err)
	require.Equal(t, expectedHash, sighashes[0].Payload)
}

func TestValidatePreparedTransactionHash_LiveCreateAccountAccept(t *testing.T) {
	t.Parallel()

	input := loadLiveAcceptInput(t)

	preparedTx, err := cantontx.BuildCreateAccountAcceptPreparedTransaction(input.SetupProposalAcceptInput)
	require.NoError(t, err)
	expectedHash, err := tx_input.ComputePreparedTransactionHash(preparedTx)
	require.NoError(t, err)
	require.NoError(t, tx_input.ValidatePreparedTransactionHash(preparedTx, expectedHash))
	require.NoError(t, input.VerifySignaturePayloads())

	args, err := xcbuilder.NewCreateAccountArgs(xc.CANTON, xc.Address(input.PartyID), []byte{0x01, 0x02})
	require.NoError(t, err)
	tx, err := cantontx.NewCreateAccountTx(args, input)
	require.NoError(t, err)

	sighashes, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, sighashes, 1)
	require.Equal(t, expectedHash, sighashes[0].Payload)
}

func TestBuildCreateAccountAcceptPreparedTransaction_LiveRoundTrip(t *testing.T) {
	t.Parallel()

	input := loadLiveAcceptInput(t)
	preparedTx := loadLegacyLiveAcceptPreparedTransaction(t)

	rebuilt, err := cantontx.BuildCreateAccountAcceptPreparedTransaction(input.SetupProposalAcceptInput)
	require.NoError(t, err)
	require.True(t, proto.Equal(preparedTx, rebuilt))
}

func hashTestPreparedTransaction(nodeID string, node *v1.Node) *interactive.PreparedTransaction {
	tx := &interactive.PreparedTransaction{
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
		Metadata: &interactive.Metadata{
			SubmitterInfo: &interactive.Metadata_SubmitterInfo{
				ActAs:     []string{"sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				CommandId: "command-id",
			},
			SynchronizerId:  "sync-id",
			TransactionUuid: "transaction-uuid",
			PreparationTime: 1,
			InputContracts:  []*interactive.Metadata_InputContract{},
		},
	}
	if node.GetExercise() != nil {
		seedID, err := strconv.Atoi(nodeID)
		if err != nil {
			panic(err)
		}
		tx.Transaction.NodeSeeds = []*interactive.DamlTransaction_NodeSeed{
			{NodeId: int32(seedID), Seed: bytes.Repeat([]byte{0x11}, 32)},
		}
	}
	return tx
}

func hashTestPreparedTransactionHash(t *testing.T, preparedTx *interactive.PreparedTransaction) []byte {
	t.Helper()
	hash, err := tx_input.ComputePreparedTransactionHash(preparedTx)
	require.NoError(t, err)
	return hash
}

func hashTestCreateNode(entity string, argument *v2.Value) *v1.Node {
	return &v1.Node{
		NodeType: &v1.Node_Create{
			Create: &v1.Create{
				TemplateId: &v2.Identifier{
					PackageId:  "pkg",
					ModuleName: "Splice.Wallet.TransferOffer",
					EntityName: entity,
				},
				ContractId:  "001122",
				PackageName: "splice-wallet",
				Argument:    argument,
			},
		},
	}
}

func hashTestExerciseNode(choice string, chosenValue *v2.Value) *v1.Node {
	return &v1.Node{
		NodeType: &v1.Node_Exercise{
			Exercise: &v1.Exercise{
				TemplateId: &v2.Identifier{
					PackageId:  "pkg",
					ModuleName: "Splice.Wallet",
					EntityName: "Any",
				},
				ContractId:  "001122",
				PackageName: "splice-wallet",
				ChoiceId:    choice,
				ChosenValue: chosenValue,
			},
		},
	}
}

func hashTestTransferOfferArgument(receiver string, amount string) *v2.Value {
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

func hashTestAmountRecord(amount string) *v2.Value {
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

func hashTestEmptyRecord() *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{},
		},
	}
}
