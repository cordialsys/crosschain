package client

import (
	"context"
	"io"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewClient(t *testing.T) {
	t.Run("missing URL", func(t *testing.T) {
		cfg := &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:  xc.CANTON,
				Driver: xc.DriverCanton,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		}
		_, err := NewClient(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no URL configured")
	})

	t.Run("missing env var", func(t *testing.T) {
		t.Setenv("CANTON_KEYCLOAK_URL", "")
		t.Setenv("CANTON_KEYCLOAK_REALM", "")
		t.Setenv("CANTON_VALIDATOR_ID", "")
		t.Setenv("CANTON_VALIDATOR_SECRET", "")
		t.Setenv("CANTON_UI_ID", "")
		t.Setenv("CANTON_UI_PASSWORD", "")
		cfg := &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:  xc.CANTON,
				Driver: xc.DriverCanton,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				URL: "https://example.com",
			},
		}
		_, err := NewClient(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "required environment variable")
	})
}

func TestSubmitTxRequiresMetadata(t *testing.T) {
	t.Parallel()

	stub := &interactiveSubmissionStub{}
	client := &Client{
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			interactiveSubmissionClient: stub,
			logger:                      logrus.NewEntry(logrus.New()),
		},
	}

	req := &interactive.ExecuteSubmissionRequest{
		SubmissionId: "submission-id",
	}
	payload, err := proto.Marshal(req)
	require.NoError(t, err)

	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "create-account payload",
			payload: mustSerializedCreateAccountInput(t),
		},
		{
			name:    "transfer payload",
			payload: payload,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := client.SubmitTx(context.Background(), xctypes.SubmitTxReq{TxData: tt.payload})
			require.ErrorContains(t, err, "missing Canton tx metadata")
			require.Nil(t, stub.lastReq)
		})
	}
}

func TestSubmitTxUsesMetadataToRouteTransferPayload(t *testing.T) {
	t.Parallel()

	stub := &interactiveSubmissionStub{}
	client := &Client{
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			interactiveSubmissionClient: stub,
			logger:                      logrus.NewEntry(logrus.New()),
		},
	}

	req := &interactive.ExecuteSubmissionRequest{SubmissionId: "submission-id"}
	payload, err := proto.Marshal(req)
	require.NoError(t, err)

	metadata, err := cantontx.NewTransferMetadata().Bytes()
	require.NoError(t, err)

	err = client.SubmitTx(context.Background(), xctypes.SubmitTxReq{
		TxData:         payload,
		BroadcastInput: string(metadata),
	})
	require.NoError(t, err)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "submission-id", stub.lastReq.GetSubmissionId())
}

func TestSubmitTxUsesMetadataToRouteCreateAccountPayload(t *testing.T) {
	t.Parallel()

	client := &Client{}
	payload, metadata := mustSerializedCreateAccountTx(t)

	err := client.SubmitTx(context.Background(), xctypes.SubmitTxReq{
		TxData:         payload,
		BroadcastInput: string(metadata),
	})
	require.ErrorContains(t, err, "create-account transaction is not signed")
}

func TestFetchTxInfoResolvesRecoveryLookupId(t *testing.T) {
	t.Parallel()

	sender := "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	updateStub := &updateServiceStub{
		resp: &v2.GetUpdateResponse{
			Update: &v2.GetUpdateResponse_Transaction{
				Transaction: &v2.Transaction{
					UpdateId:       "update-123",
					Offset:         105,
					SynchronizerId: "sync-id",
					EffectiveAt:    timestamppb.New(time.Unix(1700000000, 0)),
				},
			},
		},
	}
	completionStub := &completionServiceStub{
		responses: []*v2.CompletionStreamResponse{
			{
				CompletionResponse: &v2.CompletionStreamResponse_Completion{
					Completion: &v2.Completion{
						SubmissionId: "submission-id",
						UpdateId:     "update-123",
						Offset:       105,
					},
				},
			},
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:              "token",
			stateClient:            &stateServiceStub{ledgerEnd: 110},
			updateClient:           updateStub,
			completionClient:       completionStub,
			validatorServiceUserID: "service-account-validator-id",
			logger:                 logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("100-submission-id", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Equal(t, "update-123", info.Hash)
	require.Equal(t, "100-submission-id", info.LookupId)
	require.Equal(t, "update-123", updateStub.lastUpdateID)
	require.NotNil(t, updateStub.lastReq)
	require.Nil(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersForAnyParty())
	require.Contains(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersByParty(), sender)
	require.NotNil(t, completionStub.lastReq)
	require.Equal(t, int64(100), completionStub.lastReq.GetBeginExclusive())
	require.Equal(t, "service-account-validator-id", completionStub.lastReq.GetUserId())
	require.Equal(t, []string{sender}, completionStub.lastReq.GetParties())
}

func TestFetchTxInfoRecoveryLookupRequiresSender(t *testing.T) {
	t.Parallel()

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		ledgerClient: &GrpcLedgerClient{
			logger: logrus.NewEntry(logrus.New()),
		},
	}

	_, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("100-submission-id"))
	require.ErrorContains(t, err, "requires sender address")
}

func TestFetchTxInfoDirectUpdateLookupUsesSenderScopedRead(t *testing.T) {
	t.Parallel()

	sender := "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receiver := "receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	updateStub := &updateServiceStub{
		resp: &v2.GetUpdateResponse{
			Update: &v2.GetUpdateResponse_Transaction{
				Transaction: &v2.Transaction{
					UpdateId:       "update-123",
					Offset:         105,
					SynchronizerId: "sync-id",
					EffectiveAt:    timestamppb.New(time.Unix(1700000000, 0)),
					Events: []*v2.Event{
						{
							Event: &v2.Event_Exercised{
								Exercised: testAmuletRulesTransferEvent(sender, receiver, "20.0"),
							},
						},
					},
				},
			},
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:    "token",
			stateClient:  &stateServiceStub{ledgerEnd: 110},
			updateClient: updateStub,
			logger:       logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-123", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Equal(t, "update-123", info.Hash)
	require.Empty(t, info.LookupId)
	require.Equal(t, "sync-id/105", info.Block.Hash)
	require.Len(t, info.Movements, 2)
	require.Equal(t, xc.Address(sender), info.Movements[0].From[0].AddressId)
	require.Equal(t, xc.Address(receiver), info.Movements[0].To[0].AddressId)
	require.Len(t, info.Fees, 1)
	require.Equal(t, "3000000000000000000", info.Fees[0].Balance.String())
	require.NotNil(t, updateStub.lastReq)
	require.Equal(t, v2.TransactionShape_TRANSACTION_SHAPE_LEDGER_EFFECTS, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetTransactionShape())
	require.Nil(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersForAnyParty())
	require.Contains(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersByParty(), sender)
}

func TestFetchTxInfoUsesProvidedSenderWhenEventsDoNotExposeOne(t *testing.T) {
	t.Parallel()

	sender := "f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade::1220769b6eab2a4cc2b324e0c407b27cc7589074052c946b01aab0b1ca9b806627c6"
	updateStub := &updateServiceStub{
		resp: &v2.GetUpdateResponse{
			Update: &v2.GetUpdateResponse_Transaction{
				Transaction: &v2.Transaction{
					UpdateId:       "update-self",
					Offset:         105,
					SynchronizerId: "sync-id",
					EffectiveAt:    timestamppb.New(time.Unix(1700000000, 0)),
					Events: []*v2.Event{
						{
							Event: &v2.Event_Created{
								Created: testAmuletCreatedEvent(sender, "20.0"),
							},
						},
					},
				},
			},
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:    "token",
			stateClient:  &stateServiceStub{ledgerEnd: 110},
			updateClient: updateStub,
			logger:       logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-self", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Len(t, info.Movements, 1)
	require.Len(t, info.Movements[0].From, 1)
	require.Len(t, info.Movements[0].To, 1)
	require.Equal(t, xc.Address(sender), info.Movements[0].From[0].AddressId)
	require.Equal(t, xc.Address(sender), info.Movements[0].To[0].AddressId)
}

func TestExtractTransferFeeSupportsTransferPreapprovalSendResult(t *testing.T) {
	t.Parallel()

	ex := &v2.ExercisedEvent{
		TemplateId: &v2.Identifier{
			ModuleName: "Splice.AmuletRules",
			EntityName: "TransferPreapproval",
		},
		Choice: "TransferPreapproval_Send",
		ExerciseResult: &v2.Value{
			Sum: &v2.Value_Record{
				Record: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "result",
							Value: &v2.Value{
								Sum: &v2.Value_Record{
									Record: &v2.Record{
										Fields: []*v2.RecordField{
											{
												Label: "summary",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{
																	Label: "senderChangeFee",
																	Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: "1.5"}},
																},
																{
																	Label: "outputFees",
																	Value: &v2.Value{
																		Sum: &v2.Value_List{
																			List: &v2.List{
																				Elements: []*v2.Value{
																					{Sum: &v2.Value_Numeric{Numeric: "0.5"}},
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
						},
					},
				},
			},
		},
	}

	fee, ok := extractTransferFee(ex, 18)
	require.True(t, ok)
	require.Equal(t, "2000000000000000000", fee.String())
}

func mustSerializedCreateAccountInput(t *testing.T) []byte {
	t.Helper()
	input := &tx_input.CreateAccountInput{
		Stage:                tx_input.CreateAccountStageAllocate,
		PartyID:              "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PublicKeyFingerprint: "1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TopologyTransactions: [][]byte{{0x01}},
	}
	bz, err := input.Serialize()
	require.NoError(t, err)
	return bz
}

func mustSerializedCreateAccountTx(t *testing.T) ([]byte, []byte) {
	t.Helper()
	input := &tx_input.CreateAccountInput{
		Stage:                tx_input.CreateAccountStageAllocate,
		PartyID:              "party::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PublicKeyFingerprint: "1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TopologyTransactions: [][]byte{{0x01}},
	}
	args, err := xcbuilder.NewCreateAccountArgs(xc.CANTON, xc.Address(input.PartyID), []byte{0x01, 0x02})
	require.NoError(t, err)
	tx, err := cantontx.NewCreateAccountTx(args, input)
	require.NoError(t, err)
	payload, err := tx.Serialize()
	require.NoError(t, err)
	metadata, ok, err := tx.GetMetadata()
	require.NoError(t, err)
	require.True(t, ok)
	return payload, metadata
}

type interactiveSubmissionStub struct {
	lastReq *interactive.ExecuteSubmissionAndWaitRequest
}

func (s *interactiveSubmissionStub) PrepareSubmission(_ context.Context, _ *interactive.PrepareSubmissionRequest, _ ...grpc.CallOption) (*interactive.PrepareSubmissionResponse, error) {
	panic("unexpected call")
}
func (s *interactiveSubmissionStub) ExecuteSubmission(context.Context, *interactive.ExecuteSubmissionRequest, ...grpc.CallOption) (*interactive.ExecuteSubmissionResponse, error) {
	panic("unexpected call")
}
func (s *interactiveSubmissionStub) ExecuteSubmissionAndWait(_ context.Context, req *interactive.ExecuteSubmissionAndWaitRequest, _ ...grpc.CallOption) (*interactive.ExecuteSubmissionAndWaitResponse, error) {
	s.lastReq = req
	return &interactive.ExecuteSubmissionAndWaitResponse{}, nil
}
func (s *interactiveSubmissionStub) ExecuteSubmissionAndWaitForTransaction(context.Context, *interactive.ExecuteSubmissionAndWaitForTransactionRequest, ...grpc.CallOption) (*interactive.ExecuteSubmissionAndWaitForTransactionResponse, error) {
	panic("unexpected call")
}
func (s *interactiveSubmissionStub) GetPreferredPackageVersion(context.Context, *interactive.GetPreferredPackageVersionRequest, ...grpc.CallOption) (*interactive.GetPreferredPackageVersionResponse, error) {
	panic("unexpected call")
}
func (s *interactiveSubmissionStub) GetPreferredPackages(context.Context, *interactive.GetPreferredPackagesRequest, ...grpc.CallOption) (*interactive.GetPreferredPackagesResponse, error) {
	panic("unexpected call")
}

type completionServiceStub struct {
	lastReq   *v2.CompletionStreamRequest
	responses []*v2.CompletionStreamResponse
	streamErr error
}

func (s *completionServiceStub) CompletionStream(_ context.Context, req *v2.CompletionStreamRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[v2.CompletionStreamResponse], error) {
	s.lastReq = req
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	return &completionStreamClientStub{responses: s.responses}, nil
}

type completionStreamClientStub struct {
	grpc.ClientStream
	responses []*v2.CompletionStreamResponse
	index     int
}

func (s *completionStreamClientStub) Recv() (*v2.CompletionStreamResponse, error) {
	if s.index >= len(s.responses) {
		return nil, io.EOF
	}
	resp := s.responses[s.index]
	s.index++
	return resp, nil
}

func (s *completionStreamClientStub) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (s *completionStreamClientStub) Trailer() metadata.MD         { return metadata.MD{} }
func (s *completionStreamClientStub) CloseSend() error             { return nil }
func (s *completionStreamClientStub) Context() context.Context     { return context.Background() }
func (s *completionStreamClientStub) SendMsg(any) error            { return nil }
func (s *completionStreamClientStub) RecvMsg(any) error            { return nil }

type updateServiceStub struct {
	lastUpdateID string
	lastReq      *v2.GetUpdateByIdRequest
	resp         *v2.GetUpdateResponse
	err          error
}

func (s *updateServiceStub) GetUpdateById(_ context.Context, req *v2.GetUpdateByIdRequest, _ ...grpc.CallOption) (*v2.GetUpdateResponse, error) {
	s.lastUpdateID = req.GetUpdateId()
	s.lastReq = req
	return s.resp, s.err
}

func (s *updateServiceStub) GetUpdates(context.Context, *v2.GetUpdatesRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[v2.GetUpdatesResponse], error) {
	panic("unexpected call")
}

func (s *updateServiceStub) GetUpdateByOffset(context.Context, *v2.GetUpdateByOffsetRequest, ...grpc.CallOption) (*v2.GetUpdateResponse, error) {
	panic("unexpected call")
}

type stateServiceStub struct {
	ledgerEnd int64
}

func (s *stateServiceStub) GetActiveContracts(context.Context, *v2.GetActiveContractsRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[v2.GetActiveContractsResponse], error) {
	panic("unexpected call")
}

func (s *stateServiceStub) GetConnectedSynchronizers(context.Context, *v2.GetConnectedSynchronizersRequest, ...grpc.CallOption) (*v2.GetConnectedSynchronizersResponse, error) {
	panic("unexpected call")
}

func (s *stateServiceStub) GetLedgerEnd(context.Context, *v2.GetLedgerEndRequest, ...grpc.CallOption) (*v2.GetLedgerEndResponse, error) {
	return &v2.GetLedgerEndResponse{Offset: s.ledgerEnd}, nil
}

func (s *stateServiceStub) GetLatestPrunedOffsets(context.Context, *v2.GetLatestPrunedOffsetsRequest, ...grpc.CallOption) (*v2.GetLatestPrunedOffsetsResponse, error) {
	panic("unexpected call")
}

func testAmuletCreatedEvent(owner string, initialAmount string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: "contract-id",
		TemplateId: &v2.Identifier{
			ModuleName: "Splice.Amulet",
			EntityName: "Amulet",
		},
		CreateArguments: &v2.Record{
			Fields: []*v2.RecordField{
				{
					Label: "owner",
					Value: &v2.Value{
						Sum: &v2.Value_Party{Party: owner},
					},
				},
				{
					Label: "amount",
					Value: &v2.Value{
						Sum: &v2.Value_Record{
							Record: &v2.Record{
								Fields: []*v2.RecordField{
									{
										Label: "initialAmount",
										Value: &v2.Value{
											Sum: &v2.Value_Numeric{Numeric: initialAmount},
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

func testAmuletRulesTransferEvent(sender string, receiver string, amount string) *v2.ExercisedEvent {
	return &v2.ExercisedEvent{
		TemplateId: &v2.Identifier{
			ModuleName: "Splice.AmuletRules",
			EntityName: "AmuletRules",
		},
		Choice: "AmuletRules_Transfer",
		ActingParties: []string{
			sender,
		},
		ChoiceArgument: &v2.Value{
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
												Label: "outputs",
												Value: &v2.Value{
													Sum: &v2.Value_List{
														List: &v2.List{
															Elements: []*v2.Value{
																{
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
					},
				},
			},
		},
		ExerciseResult: &v2.Value{
			Sum: &v2.Value_Record{
				Record: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "meta",
							Value: &v2.Value{
								Sum: &v2.Value_Record{
									Record: &v2.Record{
										Fields: []*v2.RecordField{
											{
												Label: "values",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{
																	Label: "splice.lfdecentralizedtrust.org/burned",
																	Value: &v2.Value{Sum: &v2.Value_Text{Text: "3.0"}},
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
		},
	}
}
