package call_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xccall "github.com/cordialsys/crosschain/call"
	cantoncall "github.com/cordialsys/crosschain/chain/canton/call"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	v1 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive/transaction/v1"
	"github.com/stretchr/testify/require"
)

func TestSetInputRequiresPreparedTransactionToExerciseRequestedContract(t *testing.T) {
	t.Parallel()

	callTx := newTestCall(t, "001122")
	input := tx_input.NewCallInput()
	input.PreparedTransaction = testCallPreparedTransaction("1", testCallExerciseNode("aabbcc"))

	err := callTx.SetInput(input)
	require.ErrorContains(t, err, `prepared transaction does not exercise requested contract "001122"`)
}

func TestSetInputAllowsPreparedTransactionThatExercisesRequestedContract(t *testing.T) {
	t.Parallel()

	callTx := newTestCall(t, "001122")
	input := tx_input.NewCallInput()
	input.PreparedTransaction = testCallPreparedTransaction("1", testCallExerciseNode("001122"))
	input.HashingSchemeVersion = interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2
	input.SubmissionId = "submission-id"
	input.DeduplicationWindow = time.Minute

	require.NoError(t, callTx.SetInput(input))
	sighashes, err := callTx.Sighashes()
	require.NoError(t, err)
	require.Len(t, sighashes, 1)
	require.NotEmpty(t, sighashes[0].Payload)

	require.NoError(t, callTx.SetSignatures(&xc.SignatureResponse{Signature: []byte{0x01}}))
	require.Equal(t, xc.TxHash(hex.EncodeToString(sighashes[0].Payload)), callTx.Hash())
}

func newTestCall(t *testing.T, contractID string) *cantoncall.TxCall {
	t.Helper()

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: contractID})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		xccall.OfferAccept,
		payload,
		xc.Address("signer::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	)
	require.NoError(t, err)
	return callTx
}

func testCallPreparedTransaction(nodeID string, node *v1.Node) *interactive.PreparedTransaction {
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
				ActAs:     []string{"signer::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
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

func testCallExerciseNode(contractID string) *v1.Node {
	return &v1.Node{
		NodeType: &v1.Node_Exercise{
			Exercise: &v1.Exercise{
				TemplateId: &v2.Identifier{
					PackageId:  "pkg",
					ModuleName: "Splice.Wallet.TransferOffer",
					EntityName: "TransferOffer",
				},
				ContractId:  contractID,
				PackageName: "splice-wallet",
				ChoiceId:    "TransferOffer_Accept",
				ChosenValue: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{}}},
			},
		},
	}
}
