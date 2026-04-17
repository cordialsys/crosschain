package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonkc "github.com/cordialsys/crosschain/chain/canton/keycloak"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
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

	t.Run("missing custom config", func(t *testing.T) {
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
		require.Contains(t, err.Error(), "missing canton custom config field")
	})
}

func TestFetchDecimals(t *testing.T) {
	t.Parallel()

	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			scanProxyURL: "https://proxy.example",
			scanAPIURL:   "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "https://proxy.example", req.URL.String())
				require.Equal(t, "Bearer scan-token", req.Header.Get("Authorization"))

				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				require.Equal(t, http.MethodGet, envelope.Method)
				switch envelope.URL {
				case "https://scan.example/registry/metadata/v1/info":
					return httpJSONResponse(http.StatusOK, `{
						"adminId":"issuer-party",
						"supportedApis":{}
					}`), nil
				case "https://scan.example/registry/metadata/v1/instruments/Unconfigured":
					return httpJSONResponse(http.StatusOK, `{
						"id":"Unconfigured",
						"name":"Unconfigured Token",
						"symbol":"UNC",
						"decimals":6,
						"supportedApis":{}
					}`), nil
				default:
					return nil, fmt.Errorf("unexpected metadata URL %q", envelope.URL)
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("DUMMY", "", xc.ContractAddress("issuer-party#DummyHolding"), 10, xc.AmountHumanReadable{}),
	}

	decimals, err := client.FetchDecimals(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, 18, decimals)

	decimals, err = client.FetchDecimals(context.Background(), xc.ContractAddress(xc.CANTON))
	require.NoError(t, err)
	require.Equal(t, 18, decimals)

	decimals, err = client.FetchDecimals(context.Background(), xc.ContractAddress("issuer-party#DummyHolding"))
	require.NoError(t, err)
	require.Equal(t, 10, decimals)

	decimals, err = client.FetchDecimals(context.Background(), xc.ContractAddress("issuer-party#Unconfigured"))
	require.NoError(t, err)
	require.Equal(t, 6, decimals)

	_, err = client.FetchDecimals(context.Background(), xc.ContractAddress("SOME_TOKEN"))
	require.ErrorContains(t, err, "invalid Canton token contract")
}

func TestFetchDecimalsReturnsMetadataLookupErrorForUnknownCantonToken(t *testing.T) {
	t.Parallel()

	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			scanProxyURL: "https://proxy.example",
			scanAPIURL:   "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				require.Equal(t, http.MethodGet, envelope.Method)
				switch envelope.URL {
				case "https://scan.example/registry/metadata/v1/info":
					return httpJSONResponse(http.StatusOK, `{
						"adminId":"issuer-party",
						"supportedApis":{}
					}`), nil
				case "https://scan.example/registry/metadata/v1/instruments/Missing":
					return httpJSONResponse(http.StatusNotFound, `{"error":"missing"}`), nil
				default:
					return nil, fmt.Errorf("unexpected metadata URL %q", envelope.URL)
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}

	_, err := client.FetchDecimals(context.Background(), xc.ContractAddress("issuer-party#Missing"))
	require.ErrorContains(t, err, `failed to fetch token metadata for Canton token contract "issuer-party#Missing"`)
	require.ErrorContains(t, err, `status 404`)
}

func TestFetchDecimalsReturnsErrorWhenRegistryAdminDoesNotMatchInstrumentAdmin(t *testing.T) {
	t.Parallel()

	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			scanProxyURL: "https://proxy.example",
			scanAPIURL:   "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				require.Equal(t, http.MethodGet, envelope.Method)
				require.Equal(t, "https://scan.example/registry/metadata/v1/info", envelope.URL)
				return httpJSONResponse(http.StatusOK, `{
					"adminId":"other-admin",
					"supportedApis":{}
				}`), nil
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}

	_, err := client.FetchDecimals(context.Background(), xc.ContractAddress("issuer-party#XC"))
	require.ErrorContains(t, err, `registry admin "other-admin" does not match instrument admin "issuer-party"`)
}

func TestFetchBalanceTokenHolding(t *testing.T) {
	t.Parallel()

	party := xc.Address("owner-party")
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("owner-party", "issuer-party", "DummyHolding", "12.3456789012"),
					},
				},
			},
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("owner-party", "issuer-party", "OtherToken", "99.0"),
					},
				},
			},
		},
	}
	packageStub := &packageManagementStub{
		resp: &admin.ListKnownPackagesResponse{
			PackageDetails: []*admin.PackageDetails{
				{Name: "splice-api-token-holding-v1", PackageId: "holding-package-id"},
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
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:               "token",
			stateClient:             stateStub,
			packageManagementClient: packageStub,
			logger:                  logrus.NewEntry(logrus.New()),
		},
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("DUMMY", "", xc.ContractAddress("issuer-party#DummyHolding"), 10, xc.AmountHumanReadable{}),
	}

	balance, err := client.FetchBalance(context.Background(), xclient.NewBalanceArgs(party, xclient.BalanceOptionContract(xc.ContractAddress("issuer-party#DummyHolding"))))
	require.NoError(t, err)
	require.Equal(t, "123456789012", balance.String())
	require.NotNil(t, stateStub.lastActiveContractsReq)
	require.Contains(t, stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty(), "owner-party")
	filter := stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty()["owner-party"].GetCumulative()[0].GetInterfaceFilter()
	require.NotNil(t, filter)
	require.Equal(t, "holding-package-id", filter.GetInterfaceId().GetPackageId())
	require.Equal(t, "Splice.Api.Token.HoldingV1", filter.GetInterfaceId().GetModuleName())
	require.Equal(t, "Holding", filter.GetInterfaceId().GetEntityName())
	require.True(t, filter.GetIncludeInterfaceView())
}

func TestValidatorServiceUserIDFromToken(t *testing.T) {
	t.Parallel()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"preferred_username":"service-account-validator-id"}`))

	userID, err := validatorServiceUserIDFromToken(header + "." + payload + ".sig")
	require.NoError(t, err)
	require.Equal(t, "service-account-validator-id", userID)
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

func TestFetchTxInfoLeavesMovementsEmptyWhenEventsDoNotExposeSender(t *testing.T) {
	t.Parallel()

	sender := "sender::1220cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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
	require.Empty(t, info.Movements)
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

func TestExtractTransferSenderFallsBackToChoiceArgument(t *testing.T) {
	t.Parallel()

	sender := "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ex := &v2.ExercisedEvent{
		TemplateId: &v2.Identifier{
			ModuleName: "Splice.AmuletRules",
			EntityName: "TransferPreapproval",
		},
		Choice: "TransferPreapproval_Send",
		ChoiceArgument: &v2.Value{
			Sum: &v2.Value_Record{
				Record: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "sender",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}},
						},
					},
				},
			},
		},
	}

	got, ok := extractTransferSender(ex)
	require.True(t, ok)
	require.Equal(t, sender, got)
}

func TestBuildTransferOfferCreateCommandUsesArgsAmount(t *testing.T) {
	t.Parallel()

	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
	)
	require.NoError(t, err)

	cmd := buildTransferOfferCreateCommand(args, AmuletRules{
		AmuletRulesUpdate: struct {
			Contract AmuletRulesContract `json:"contract"`
			DomainID string              `json:"domain_id"`
		}{
			Contract: AmuletRulesContract{
				TemplateID: "pkg-from-amulet-rules:Splice.AmuletRules:AmuletRules",
				Payload: struct {
					DSO string `json:"dso"`
				}{DSO: "validator-party"},
			},
		},
	}, "command-id", 1)

	create := cmd.GetCreate()
	require.NotNil(t, create)
	require.Equal(t, "pkg-from-amulet-rules", create.GetTemplateId().GetPackageId())
	require.Equal(t, "12.3", extractCommandAmountNumeric(t, create.GetCreateArguments()))
}

func TestBuildTransferPreapprovalExerciseCommandUsesArgsAmount(t *testing.T) {
	t.Parallel()

	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
	)
	require.NoError(t, err)

	cmd, _, err := buildTransferPreapprovalExerciseCommand(
		args,
		AmuletRules{
			AmuletRulesUpdate: struct {
				Contract AmuletRulesContract `json:"contract"`
				DomainID string              `json:"domain_id"`
			}{
				Contract: AmuletRulesContract{
					ContractID: "amulet-rules-contract",
					TemplateID: "pkg:Module:AmuletRules",
					Payload: struct {
						DSO string `json:"dso"`
					}{DSO: "validator-party"},
				},
			},
		},
		&RoundEntry{Contract: RoundContract{ContractID: "open-round", TemplateID: "pkg:Module:OpenRound"}},
		&RoundEntry{Contract: RoundContract{
			ContractID: "issuing-round",
			TemplateID: "pkg:Module:IssuingRound",
			Payload: RoundPayload{Round: struct {
				Number string `json:"number"`
			}{Number: "1"}},
		}},
		[]*v2.ActiveContract{
			{
				CreatedEvent: &v2.CreatedEvent{
					ContractId: "sender-amulet",
					TemplateId: &v2.Identifier{EntityName: "Amulet"},
				},
			},
		},
		[]*v2.ActiveContract{
			{
				CreatedEvent: &v2.CreatedEvent{
					ContractId:       "preapproval-contract",
					CreatedEventBlob: []byte{0x01},
					TemplateId: &v2.Identifier{
						ModuleName: "Splice.AmuletRules",
						EntityName: "TransferPreapproval",
					},
				},
			},
		},
		1,
	)
	require.NoError(t, err)

	exercise := cmd.GetExercise()
	require.NotNil(t, exercise)
	require.Equal(t, "12.3", extractCommandAmountNumeric(t, exercise.GetChoiceArgument().GetRecord()))
}

func TestBuildTokenStandardTransferCommandUsesArgsAmount(t *testing.T) {
	t.Parallel()

	contract := xc.ContractAddress("issuer-party#XC")
	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
		xcbuilder.OptionContractAddress(contract),
	)
	require.NoError(t, err)

	cmd, err := buildTokenStandardTransferCommand(
		args,
		"transfer-package-id",
		"factory-cid",
		map[string]any{
			"values": map[string]any{
				"factoryId": map[string]any{
					"tag":   "AV_ContractId",
					"value": "factory-cid",
				},
			},
		},
		[]*v2.ActiveContract{
			{CreatedEvent: testTokenHoldingCreatedEventWithContractID("holding-cid", string(args.GetFrom()), "issuer-party", "XC", "100.0")},
		},
		1,
		time.Unix(1700000000, 123000000).UTC(),
		time.Unix(1700086400, 456000000).UTC(),
	)
	require.NoError(t, err)

	exercise := cmd.GetExercise()
	require.NotNil(t, exercise)
	require.Equal(t, "transfer-package-id", exercise.GetTemplateId().GetPackageId())
	require.Equal(t, tokenTransferModule, exercise.GetTemplateId().GetModuleName())
	require.Equal(t, tokenTransferEntity, exercise.GetTemplateId().GetEntityName())
	require.Equal(t, "factory-cid", exercise.GetContractId())
	require.Equal(t, "TransferFactory_Transfer", exercise.GetChoice())

	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	require.NotNil(t, choiceArgument)
	transferValue, ok := getRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	require.NotNil(t, transferValue.GetRecord())
	require.Equal(t, "12.3", extractCommandAmountNumeric(t, transferValue.GetRecord()))
	requestedAtValue, ok := getRecordFieldValue(transferValue.GetRecord(), "requestedAt")
	require.True(t, ok)
	require.Equal(t, time.Unix(1700000000, 123000000).UTC().UnixMicro(), requestedAtValue.GetTimestamp())
	executeBeforeValue, ok := getRecordFieldValue(transferValue.GetRecord(), "executeBefore")
	require.True(t, ok)
	require.Equal(t, time.Unix(1700086400, 456000000).UTC().UnixMicro(), executeBeforeValue.GetTimestamp())

	instrumentValue, ok := getRecordFieldValue(transferValue.GetRecord(), "instrumentId")
	require.True(t, ok)
	require.Equal(t, "issuer-party", instrumentValue.GetRecord().GetFields()[0].GetValue().GetParty())
	require.Equal(t, "XC", instrumentValue.GetRecord().GetFields()[1].GetValue().GetText())

	inputHoldingValue, ok := getRecordFieldValue(transferValue.GetRecord(), "inputHoldingCids")
	require.True(t, ok)
	require.Len(t, inputHoldingValue.GetList().GetElements(), 1)
	require.Equal(t, "holding-cid", inputHoldingValue.GetList().GetElements()[0].GetContractId())

	extraArgsValue, ok := getRecordFieldValue(choiceArgument, "extraArgs")
	require.True(t, ok)
	contextValue, ok := getRecordFieldValue(extraArgsValue.GetRecord(), "context")
	require.True(t, ok)
	valuesField, ok := getRecordFieldValue(contextValue.GetRecord(), "values")
	require.True(t, ok)
	require.Len(t, valuesField.GetTextMap().GetEntries(), 1)
}

func TestFetchTransferInputTokenStandard(t *testing.T) {
	t.Parallel()

	contract := xc.ContractAddress("issuer-party#XC")
	var registryChoiceArgs map[string]any
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction: &interactive.PreparedTransaction{
			Transaction: &interactive.DamlTransaction{},
		},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEventWithContractID("holding-cid", "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "issuer-party", "XC", "100.0"),
					},
				},
			},
		},
	}
	packageStub := &packageManagementStub{
		resp: &admin.ListKnownPackagesResponse{
			PackageDetails: []*admin.PackageDetails{
				{Name: "splice-api-token-holding-v1", PackageId: "holding-package-id"},
				{Name: "splice-api-token-transfer-instruction-v1", PackageId: "transfer-package-id"},
			},
		},
	}
	interactiveStub := &interactiveSubmissionStub{prepareResp: prepareResp}
	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			packageManagementClient:     packageStub,
			interactiveSubmissionClient: interactiveStub,
			scanProxyURL:                "https://proxy.example",
			scanAPIURL:                  "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://proxy.example", req.URL.String())
				require.Equal(t, "Bearer scan-token", req.Header.Get("Authorization"))

				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				require.Equal(t, "https://scan.example/registry/transfer-instruction/v1/transfer-factory", envelope.URL)

				var requestBody map[string]any
				require.NoError(t, json.Unmarshal([]byte(envelope.Body), &requestBody))
				require.Contains(t, requestBody, "choiceArguments")
				choiceArgs, ok := requestBody["choiceArguments"].(map[string]any)
				require.True(t, ok)
				registryChoiceArgs = choiceArgs

				body := `{
					"factoryId":"factory-cid",
					"transferKind":"offer",
					"choiceContext":{
						"choiceContextData":{
							"values":{
								"factoryRef":{"tag":"AV_ContractId","value":"factory-cid"}
							}
						},
						"disclosedContracts":[
							{
								"templateId":"#splice-api-token-transfer-instruction-v1:Splice.Api.Token.TransferInstructionV1:TransferFactory",
								"contractId":"factory-cid",
								"createdEventBlob":"AQ==",
								"synchronizerId":"sync-id"
							}
						]
					}
				}`
				return httpJSONResponse(http.StatusOK, body), nil
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("XC", "", contract, 10, xc.AmountHumanReadable{}),
	}

	args, err := xcbuilder.NewTransferArgs(
		client.Asset.GetChain().Base(),
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
		xcbuilder.OptionContractAddress(contract),
	)
	require.NoError(t, err)

	inputI, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)

	input, ok := inputI.(*tx_input.TxInput)
	require.True(t, ok)
	require.Equal(t, int32(10), input.Decimals)
	require.Equal(t, int64(123), input.LedgerEnd)
	require.Equal(t, prepareResp.GetPreparedTransaction(), input.PreparedTransaction)
	require.NotNil(t, interactiveStub.lastPrepareReq)
	require.Equal(t, []string{string(args.GetFrom())}, interactiveStub.lastPrepareReq.GetActAs())
	require.Equal(t, "transfer-package-id", interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise().GetTemplateId().GetPackageId())
	require.Equal(t, "TransferFactory_Transfer", interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise().GetChoice())
	require.Len(t, interactiveStub.lastPrepareReq.GetDisclosedContracts(), 1)
	require.NotNil(t, registryChoiceArgs)
	registryTransfer, ok := registryChoiceArgs["transfer"].(map[string]any)
	require.True(t, ok)
	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	transferValue, ok := getRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	requestedAtValue, ok := getRecordFieldValue(transferValue.GetRecord(), "requestedAt")
	require.True(t, ok)
	require.Equal(t, registryTransfer["requestedAt"], time.UnixMicro(requestedAtValue.GetTimestamp()).UTC().Format(time.RFC3339Nano))
	executeBeforeValue, ok := getRecordFieldValue(transferValue.GetRecord(), "executeBefore")
	require.True(t, ok)
	require.Equal(t, registryTransfer["executeBefore"], time.UnixMicro(executeBeforeValue.GetTimestamp()).UTC().Format(time.RFC3339Nano))
}

func TestFetchTransferInputTokenStandardUsesExplicitArgDecimals(t *testing.T) {
	t.Parallel()

	contract := xc.ContractAddress("issuer-party#XC")
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction: &interactive.PreparedTransaction{
			Transaction: &interactive.DamlTransaction{},
		},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEventWithContractID("holding-cid", "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "issuer-party", "XC", "100.0"),
					},
				},
			},
		},
	}
	packageStub := &packageManagementStub{
		resp: &admin.ListKnownPackagesResponse{
			PackageDetails: []*admin.PackageDetails{
				{Name: "splice-api-token-holding-v1", PackageId: "holding-package-id"},
				{Name: "splice-api-token-transfer-instruction-v1", PackageId: "transfer-package-id"},
			},
		},
	}
	interactiveStub := &interactiveSubmissionStub{prepareResp: prepareResp}
	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			packageManagementClient:     packageStub,
			interactiveSubmissionClient: interactiveStub,
			scanProxyURL:                "https://proxy.example",
			scanAPIURL:                  "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				body := `{
					"factoryId":"factory-cid",
					"transferKind":"offer",
					"choiceContext":{
						"choiceContextData":{"values":{}},
						"disclosedContracts":[
							{
								"templateId":"#splice-api-token-transfer-instruction-v1:Splice.Api.Token.TransferInstructionV1:TransferFactory",
								"contractId":"factory-cid",
								"createdEventBlob":"AQ==",
								"synchronizerId":"sync-id"
							}
						]
					}
				}`
				return httpJSONResponse(http.StatusOK, body), nil
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}

	args, err := xcbuilder.NewTransferArgs(
		client.Asset.GetChain().Base(),
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(1_000_000),
		xcbuilder.OptionContractAddress(contract),
		xcbuilder.OptionContractDecimals(6),
	)
	require.NoError(t, err)

	inputI, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	input := inputI.(*tx_input.TxInput)
	require.Equal(t, int32(6), input.Decimals)

	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	transferValue, ok := getRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	require.Equal(t, "1", extractCommandAmountNumeric(t, transferValue.GetRecord()))
}

func TestFetchTransferInputTokenStandardFallsBackToLedgerFactory(t *testing.T) {
	t.Parallel()

	contract := xc.ContractAddress("issuer-party#XC")
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction: &interactive.PreparedTransaction{
			Transaction: &interactive.DamlTransaction{},
		},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenTransferFactoryCreatedEvent("factory-cid", "issuer-party", "XC"),
					},
				},
			},
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEventWithContractID("holding-cid", "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "issuer-party", "XC", "100.0"),
					},
				},
			},
		},
	}
	packageStub := &packageManagementStub{
		resp: &admin.ListKnownPackagesResponse{
			PackageDetails: []*admin.PackageDetails{
				{Name: "splice-api-token-holding-v1", PackageId: "holding-package-id"},
				{Name: "splice-api-token-transfer-instruction-v1", PackageId: "transfer-package-id"},
			},
		},
	}
	interactiveStub := &interactiveSubmissionStub{
		prepareResponses: []*interactive.PrepareSubmissionResponse{nil, prepareResp},
		prepareErrors: []error{
			fmt.Errorf("rpc error: code = FailedPrecondition desc = expected admin mismatch"),
			nil,
		},
	}
	keycloakServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scan-token",
			"expires_in":   300,
		}))
	}))
	defer keycloakServer.Close()
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			packageManagementClient:     packageStub,
			interactiveSubmissionClient: interactiveStub,
			scanProxyURL:                "https://proxy.example",
			scanAPIURL:                  "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body := `{
					"factoryId":"wrong-factory-cid",
					"transferKind":"offer",
					"choiceContext":{
						"choiceContextData":{"values":{}},
						"disclosedContracts":[
							{
								"templateId":"#splice-api-token-transfer-instruction-v1:Splice.Api.Token.TransferInstructionV1:TransferFactory",
								"contractId":"wrong-factory-cid",
								"createdEventBlob":"AQ==",
								"synchronizerId":"sync-id"
							}
						]
					}
				}`
				return httpJSONResponse(http.StatusOK, body), nil
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}

	args, err := xcbuilder.NewTransferArgs(
		client.Asset.GetChain().Base(),
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
		xcbuilder.OptionContractAddress(contract),
		xcbuilder.OptionContractDecimals(1),
	)
	require.NoError(t, err)

	inputI, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, inputI)
	require.Equal(t, 2, interactiveStub.prepareCalls)
	require.NotNil(t, interactiveStub.lastPrepareReq)
	require.Equal(t, "factory-cid", interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise().GetContractId())
	require.Len(t, interactiveStub.lastPrepareReq.GetDisclosedContracts(), 1)
	require.Equal(t, []string{string(args.GetFrom()), "issuer-party"}, interactiveStub.lastPrepareReq.GetReadAs())
}

func TestGetAmuletRulesUsesStructuredScanProxyRequest(t *testing.T) {
	client := &GrpcLedgerClient{
		scanProxyURL: "https://proxy.example",
		scanAPIURL:   "https://scan.example",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://proxy.example", req.URL.String())
			require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			var envelope scanProxyRequest
			require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
			require.Equal(t, "POST", envelope.Method)
			require.Equal(t, "https://scan.example/api/scan/v0/amulet-rules", envelope.URL)
			require.Equal(t, "application/json", envelope.Headers["Content-Type"])
			require.JSONEq(t, `{}`, envelope.Body)

			body := `{"amulet_rules_update":{"contract":{"template_id":"pkg:Mod:AmuletRules","contract_id":"cid","created_event_blob":"AQ==","payload":{"dso":"dso"}},"domain_id":"domain"}}`
			return httpJSONResponse(http.StatusOK, body), nil
		})},
	}

	result, err := client.GetAmuletRules(context.Background(), "token")
	require.NoError(t, err)
	require.Equal(t, "domain", result.AmuletRulesUpdate.DomainID)
}

func TestGetOpenAndIssuingMiningRoundUsesStructuredScanProxyRequest(t *testing.T) {
	client := &GrpcLedgerClient{
		scanProxyURL: "https://proxy.example",
		scanAPIURL:   "https://scan.example/",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var envelope scanProxyRequest
			require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
			require.Equal(t, "POST", envelope.Method)
			require.Equal(t, "https://scan.example/api/scan/v0/open-and-issuing-mining-rounds", envelope.URL)
			require.Equal(t, "application/json", envelope.Headers["Content-Type"])

			var inner map[string][]string
			require.NoError(t, json.Unmarshal([]byte(envelope.Body), &inner))
			require.Contains(t, inner, "cached_open_mining_round_contract_ids")
			require.Contains(t, inner, "cached_issuing_round_contract_ids")
			require.Empty(t, inner["cached_open_mining_round_contract_ids"])
			require.Empty(t, inner["cached_issuing_round_contract_ids"])

			body := `{"open_mining_rounds":{"open":{"contract":{"contract_id":"open-cid","template_id":"pkg:Mod:Open","payload":{"round":{"number":"1"},"opensAt":"2000-01-01T00:00:00Z","targetClosesAt":"2999-01-01T00:00:00Z"},"created_event_blob":"AQ=="}, "domain_id":"domain"}},"issuing_mining_rounds":{"issuing":{"contract":{"contract_id":"issuing-cid","template_id":"pkg:Mod:Issuing","payload":{"round":{"number":"1"},"opensAt":"2000-01-01T00:00:00Z","targetClosesAt":"2999-01-01T00:00:00Z"},"created_event_blob":"AQ=="},"domain_id":"domain"}}}`
			return httpJSONResponse(http.StatusOK, body), nil
		})},
	}

	open, issuing, err := client.GetOpenAndIssuingMiningRound(context.Background(), "token")
	require.NoError(t, err)
	require.Equal(t, "open-cid", open.Contract.ContractID)
	require.Equal(t, "issuing-cid", issuing.Contract.ContractID)
}

func TestGetAmuletRulesPreservesScanProxyHTTPError(t *testing.T) {
	client := &GrpcLedgerClient{
		scanProxyURL: "https://proxy.example",
		scanAPIURL:   "https://scan.example",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return httpJSONResponse(http.StatusBadGateway, `upstream failed`), nil
		})},
	}

	_, err := client.GetAmuletRules(context.Background(), "token")
	require.ErrorContains(t, err, "fetching amulet rules: status 502: upstream failed")
}

func mustSerializedCreateAccountInput(t *testing.T) []byte {
	t.Helper()
	input := &tx_input.CreateAccountInput{
		Stage:                tx_input.CreateAccountStageAllocate,
		PartyID:              "e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8",
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
		PartyID:              "e5c86207770b9fb67d73eb4cb8cd6a5f6a5d6a63c66a5459bd77cca45fda6ede::122079aa518eac66dcd662887155c5c7ee36d3b62e38ed0ded2ddc0c7050460bccc8",
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
	lastPrepareReq   *interactive.PrepareSubmissionRequest
	prepareResp      *interactive.PrepareSubmissionResponse
	prepareErr       error
	prepareResponses []*interactive.PrepareSubmissionResponse
	prepareErrors    []error
	prepareCalls     int
	lastReq          *interactive.ExecuteSubmissionAndWaitRequest
}

func (s *interactiveSubmissionStub) PrepareSubmission(_ context.Context, req *interactive.PrepareSubmissionRequest, _ ...grpc.CallOption) (*interactive.PrepareSubmissionResponse, error) {
	s.lastPrepareReq = req
	if s.prepareCalls < len(s.prepareResponses) || s.prepareCalls < len(s.prepareErrors) {
		var resp *interactive.PrepareSubmissionResponse
		var err error
		if s.prepareCalls < len(s.prepareResponses) {
			resp = s.prepareResponses[s.prepareCalls]
		}
		if s.prepareCalls < len(s.prepareErrors) {
			err = s.prepareErrors[s.prepareCalls]
		}
		s.prepareCalls++
		return resp, err
	}
	if s.prepareResp != nil || s.prepareErr != nil {
		s.prepareCalls++
		return s.prepareResp, s.prepareErr
	}
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
	ledgerEnd                  int64
	lastActiveContractsReq     *v2.GetActiveContractsRequest
	activeContractsResponses   []*v2.GetActiveContractsResponse
	activeContractsErr         error
	connectedSynchronizersResp *v2.GetConnectedSynchronizersResponse
	connectedSynchronizersErr  error
}

func (s *stateServiceStub) GetActiveContracts(_ context.Context, req *v2.GetActiveContractsRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[v2.GetActiveContractsResponse], error) {
	s.lastActiveContractsReq = req
	if s.activeContractsErr != nil {
		return nil, s.activeContractsErr
	}
	return &activeContractsStreamStub{responses: s.activeContractsResponses}, nil
}

func (s *stateServiceStub) GetConnectedSynchronizers(_ context.Context, req *v2.GetConnectedSynchronizersRequest, _ ...grpc.CallOption) (*v2.GetConnectedSynchronizersResponse, error) {
	if s.connectedSynchronizersErr != nil {
		return nil, s.connectedSynchronizersErr
	}
	if s.connectedSynchronizersResp != nil {
		return s.connectedSynchronizersResp, nil
	}
	return &v2.GetConnectedSynchronizersResponse{
		ConnectedSynchronizers: []*v2.GetConnectedSynchronizersResponse_ConnectedSynchronizer{
			{
				SynchronizerId: "sync-id",
				Permission:     v2.ParticipantPermission_PARTICIPANT_PERMISSION_SUBMISSION,
			},
		},
	}, nil
}

func (s *stateServiceStub) GetLedgerEnd(context.Context, *v2.GetLedgerEndRequest, ...grpc.CallOption) (*v2.GetLedgerEndResponse, error) {
	return &v2.GetLedgerEndResponse{Offset: s.ledgerEnd}, nil
}

func (s *stateServiceStub) GetLatestPrunedOffsets(context.Context, *v2.GetLatestPrunedOffsetsRequest, ...grpc.CallOption) (*v2.GetLatestPrunedOffsetsResponse, error) {
	panic("unexpected call")
}

type activeContractsStreamStub struct {
	grpc.ClientStream
	responses []*v2.GetActiveContractsResponse
	index     int
}

func (s *activeContractsStreamStub) Recv() (*v2.GetActiveContractsResponse, error) {
	if s.index >= len(s.responses) {
		return nil, io.EOF
	}
	resp := s.responses[s.index]
	s.index++
	return resp, nil
}

func (s *activeContractsStreamStub) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (s *activeContractsStreamStub) Trailer() metadata.MD         { return metadata.MD{} }
func (s *activeContractsStreamStub) CloseSend() error             { return nil }
func (s *activeContractsStreamStub) Context() context.Context     { return context.Background() }
func (s *activeContractsStreamStub) SendMsg(any) error            { return nil }
func (s *activeContractsStreamStub) RecvMsg(any) error            { return nil }

type packageManagementStub struct {
	resp *admin.ListKnownPackagesResponse
	err  error
}

func (s *packageManagementStub) ListKnownPackages(context.Context, *admin.ListKnownPackagesRequest, ...grpc.CallOption) (*admin.ListKnownPackagesResponse, error) {
	return s.resp, s.err
}

func (s *packageManagementStub) UploadDarFile(context.Context, *admin.UploadDarFileRequest, ...grpc.CallOption) (*admin.UploadDarFileResponse, error) {
	panic("unexpected call")
}

func (s *packageManagementStub) ValidateDarFile(context.Context, *admin.ValidateDarFileRequest, ...grpc.CallOption) (*admin.ValidateDarFileResponse, error) {
	panic("unexpected call")
}

func (s *packageManagementStub) UpdateVettedPackages(context.Context, *admin.UpdateVettedPackagesRequest, ...grpc.CallOption) (*admin.UpdateVettedPackagesResponse, error) {
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

func testTokenHoldingCreatedEvent(owner string, issuer string, instrumentID string, amount string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: "token-contract-id",
		InterfaceViews: []*v2.InterfaceView{
			{
				InterfaceId: &v2.Identifier{
					PackageId:  "holding-package-id",
					ModuleName: "Splice.Api.Token.HoldingV1",
					EntityName: "Holding",
				},
				ViewValue: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "owner",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: owner}},
						},
						{
							Label: "instrumentId",
							Value: &v2.Value{
								Sum: &v2.Value_Record{
									Record: &v2.Record{
										Fields: []*v2.RecordField{
											{
												Label: "admin",
												Value: &v2.Value{Sum: &v2.Value_Party{Party: issuer}},
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
						{
							Label: "amount",
							Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}},
						},
					},
				},
			},
		},
	}
}

func testTokenHoldingCreatedEventWithContractID(contractID string, owner string, issuer string, instrumentID string, amount string) *v2.CreatedEvent {
	event := testTokenHoldingCreatedEvent(owner, issuer, instrumentID, amount)
	event.ContractId = contractID
	return event
}

func testTokenTransferFactoryCreatedEvent(contractID string, admin string, instrumentID string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: contractID,
		CreateArguments: &v2.Record{
			Fields: []*v2.RecordField{
				{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
				{Label: "symbol", Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}}},
			},
		},
		InterfaceViews: []*v2.InterfaceView{
			{
				InterfaceId: &v2.Identifier{
					PackageId:  "transfer-package-id",
					ModuleName: "Splice.Api.Token.TransferInstructionV1",
					EntityName: "TransferFactory",
				},
				ViewValue: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "admin",
							Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}},
						},
					},
				},
			},
		},
	}
}

func extractCommandAmountNumeric(t *testing.T, record *v2.Record) string {
	t.Helper()

	for _, field := range record.GetFields() {
		if field.GetLabel() != "amount" {
			continue
		}
		if numeric := field.GetValue().GetNumeric(); numeric != "" {
			return numeric
		}
		if amountRecord := field.GetValue().GetRecord(); amountRecord != nil {
			for _, nested := range amountRecord.GetFields() {
				if nested.GetLabel() == "amount" {
					return nested.GetValue().GetNumeric()
				}
			}
		}
	}
	t.Fatalf("amount field not found")
	return ""
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
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
