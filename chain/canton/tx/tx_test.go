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
)

func TestNewTx_UsesPreparedTransactionForTransferFlows(t *testing.T) {
	t.Parallel()

	from := xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	to := xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	amount := xc.NewAmountBlockchainFromUint64(100)
	chainCfg := &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton}

	vectors := []struct {
		name       string
		args       xcbuilder.TransferArgs
		preparedTx *interactive.PreparedTransaction
	}{
		{
			name: "transfer_offer",
			args: func() xcbuilder.TransferArgs {
				args, err := xcbuilder.NewTransferArgs(chainCfg, from, to, amount)
				if err != nil {
					panic(err)
				}
				return args
			}(),
			preparedTx: txPreparedTransaction("1", txCreateNode("TransferOffer", txTransferOfferArgument(string(to), "10.0"))),
		},
		{
			name: "transfer_preapproval_send",
			args: func() xcbuilder.TransferArgs {
				args, err := xcbuilder.NewTransferArgs(chainCfg, from, to, amount)
				if err != nil {
					panic(err)
				}
				return args
			}(),
			preparedTx: txPreparedTransaction("1", txExerciseNode("TransferPreapproval_Send", txAmountRecord("10.0"))),
		},
		{
			name: "token_transfer_factory",
			args: func() xcbuilder.TransferArgs {
				args, err := xcbuilder.NewTransferArgs(chainCfg, from, to, amount, xcbuilder.OptionContractAddress(xc.ContractAddress("issuer-party#XC")))
				if err != nil {
					panic(err)
				}
				return args
			}(),
			preparedTx: txPreparedTransaction("1", txExerciseNode("TransferFactory_Transfer", txTransferFactoryArgument(string(to), "issuer-party", "XC", "10.0"))),
		},
	}

	for _, vector := range vectors {
		vector := vector
		t.Run(vector.name, func(t *testing.T) {
			t.Parallel()

			input := &tx_input.TxInput{
				PreparedTransaction: vector.preparedTx,
				LedgerEnd:           12345,
				SubmissionId:        "submission-id",
			}

			tx, err := cantontx.NewTx(input, vector.args, 1)
			require.NoError(t, err)

			hash, err := tx_input.ComputePreparedTransactionHash(vector.preparedTx)
			require.NoError(t, err)

			sighashes, err := tx.Sighashes()
			require.NoError(t, err)
			require.Len(t, sighashes, 1)
			require.Equal(t, hash, sighashes[0].Payload)
			require.Equal(t, xc.TxHash("12345-submission-id"), tx.Hash())
		})
	}
}

func TestNewTx_RejectsTokenTransferWithMismatchedContract(t *testing.T) {
	t.Parallel()

	from := xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	to := xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		from,
		to,
		xc.NewAmountBlockchainFromUint64(100),
		xcbuilder.OptionContractAddress(xc.ContractAddress("issuer-party#XC")),
	)
	require.NoError(t, err)

	input := &tx_input.TxInput{
		PreparedTransaction: txPreparedTransaction("1", txExerciseNode("TransferFactory_Transfer", txTransferFactoryArgument(string(to), "other-issuer", "XC", "10.0"))),
		LedgerEnd:           12345,
		SubmissionId:        "submission-id",
	}

	_, err = cantontx.NewTx(input, args, 1)
	require.ErrorContains(t, err, "instrument admin mismatch")
}

func txTransferFactoryArgument(receiver string, admin string, instrumentID string, amount string) *v2.Value {
	return &v2.Value{
		Sum: &v2.Value_Record{
			Record: &v2.Record{
				Fields: []*v2.RecordField{
					{
						Label: "transfer",
						Value: &v2.Value{
							Sum: &v2.Value_Record{
								Record: &v2.Record{
									Fields: []*v2.RecordField{
										{
											Label: "receiver",
											Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}},
										},
										{
											Label: "amount",
											Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}},
										},
										{
											Label: "instrumentId",
											Value: &v2.Value{
												Sum: &v2.Value_Record{
													Record: &v2.Record{
														Fields: []*v2.RecordField{
															{
																Label: "admin",
																Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}},
															},
															{
																Label: "id",
																Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}},
															},
														},
													},
												},
											},
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

func txPreparedTransaction(nodeID string, node *v1.Node) *interactive.PreparedTransaction {
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

func txCreateNode(entity string, argument *v2.Value) *v1.Node {
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

func txExerciseNode(choice string, chosenValue *v2.Value) *v1.Node {
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

func txTransferOfferArgument(receiver string, amount string) *v2.Value {
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

func txAmountRecord(amount string) *v2.Value {
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
