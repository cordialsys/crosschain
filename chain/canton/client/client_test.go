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
	xccall "github.com/cordialsys/crosschain/call"
	cantoncall "github.com/cordialsys/crosschain/chain/canton/call"
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"): "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"): "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("issuer-party#XC"): "https://registry.example",
			},
		},
		ledgerClient: &GrpcLedgerClient{
			scanProxyURL: "https://proxy.example",
			scanAPIURL:   "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "https://registry.example/registry/metadata/v1/info", req.URL.String())
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
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("other-owner", "issuer-party", "DummyHolding", "88.0"),
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("issuer-party#DummyHolding"): "https://registry.example",
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:               "token",
			stateClient:             stateStub,
			packageManagementClient: packageStub,
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "", req.Header.Get("Authorization"))
				switch req.URL.String() {
				case "https://registry.example/registry/metadata/v1/info":
					return httpJSONResponse(http.StatusOK, `{
						"adminId":"issuer-party",
						"supportedApis":{}
					}`), nil
				case "https://registry.example/registry/metadata/v1/instruments/DummyHolding":
					return httpJSONResponse(http.StatusOK, `{
						"id":"DummyHolding",
						"name":"Dummy Holding",
						"symbol":"DUMMY",
						"decimals":6,
						"supportedApis":{}
					}`), nil
				default:
					return nil, fmt.Errorf("unexpected registry URL %q", req.URL.String())
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
	}

	balance, err := client.FetchBalance(context.Background(), xclient.NewBalanceArgs(party, xclient.BalanceOptionContract(xc.ContractAddress("issuer-party#DummyHolding"))))
	require.NoError(t, err)
	require.Equal(t, "12345678", balance.String())
	require.NotNil(t, stateStub.lastActiveContractsReq)
	require.Contains(t, stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty(), "owner-party")
	filter := stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty()["owner-party"].GetCumulative()[0].GetInterfaceFilter()
	require.NotNil(t, filter)
	require.Equal(t, "holding-package-id", filter.GetInterfaceId().GetPackageId())
	require.Equal(t, "Splice.Api.Token.HoldingV1", filter.GetInterfaceId().GetModuleName())
	require.Equal(t, "Holding", filter.GetInterfaceId().GetEntityName())
	require.True(t, filter.GetIncludeInterfaceView())
}

func TestFetchDecimalsResolvesConfiguredRegistryByContract(t *testing.T) {
	t.Parallel()

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 18,
				Network:  string(xc.NotMainnets),
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"): "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		ledgerClient: &GrpcLedgerClient{
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "", req.Header.Get("Authorization"))
				switch req.URL.String() {
				case "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f/registry/metadata/v1/info":
					return httpJSONResponse(http.StatusOK, `{
						"adminId":"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
						"supportedApis":{}
					}`), nil
				case "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f/registry/metadata/v1/instruments/CBTC":
					return httpJSONResponse(http.StatusOK, `{
						"id":"CBTC",
						"name":"CBTC",
						"symbol":"CBTC",
						"decimals":8,
						"supportedApis":{}
					}`), nil
				default:
					return nil, fmt.Errorf("unexpected utilities registry URL %q", req.URL.String())
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
	}

	decimals, err := client.FetchDecimals(context.Background(), xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"))
	require.NoError(t, err)
	require.Equal(t, 8, decimals)
}

func TestListPendingOffers(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 42,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletOfferContract(
				"pending-1",
				"TransferOffer",
				"owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("10.0000000000"),
				"tracking-1",
				time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletOfferContract(
				"pending-unrelated",
				"TransferOffer",
				"other-sender",
				"other-receiver",
				cantonAmuletOfferAmount("1.0000000000"),
				"tracking-unrelated",
				time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletOfferContract(
				"accepted-1",
				"AcceptedTransferOffer",
				"owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("5.0000000000"),
				"tracking-accepted",
				time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(context.Background(), xclient.NewOfferArgs(xc.Address("owner-party")))
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "pending-1", offers[0].ID)
	require.Equal(t, xc.ContractAddress(xc.CANTON), offers[0].AssetID)
	require.Equal(t, xc.Address("owner-party"), offers[0].From)
	require.Equal(t, xc.Address("receiver-party"), offers[0].To)
	require.Equal(t, "100000000000", offers[0].Amount.String())
	require.Equal(t, "tracking-1", offers[0].TrackingID)
	require.NotNil(t, offers[0].ExpiresAt)
	require.NotNil(t, stateStub.lastActiveContractsReq)
	require.Contains(t, stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty(), "owner-party")
}

func TestListPendingOffersFiltersByTokenContract(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 7,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletTransferRecordContract(
				"token-pending",
				"TransferOffer",
				"owner-party",
				"receiver-party",
				"issuer-party",
				"XC",
				"10.500000",
				"tracking-token",
				time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletTransferRecordContract(
				"other-token",
				"TransferOffer",
				"owner-party",
				"receiver-party",
				"issuer-party",
				"OTHER",
				"2.000000",
				"tracking-other",
				time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("XC", "", xc.ContractAddress("issuer-party#XC"), 6, xc.AmountHumanReadable{}),
		xc.NewAdditionalNativeAsset("OTHER", "", xc.ContractAddress("issuer-party#OTHER"), 6, xc.AmountHumanReadable{}),
	}

	offers, err := client.ListPendingOffers(
		context.Background(),
		xclient.NewOfferArgs(xc.Address("owner-party"), xclient.OfferOptionContract(xc.ContractAddress("issuer-party#XC"))),
	)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "token-pending", offers[0].ID)
	require.Equal(t, xc.ContractAddress("issuer-party#XC"), offers[0].AssetID)
	require.Equal(t, "10500000", offers[0].Amount.String())
}

func TestListPendingOffersIncludesUtilitiesTransferOffer(t *testing.T) {
	t.Parallel()

	executeBefore := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	stateStub := &stateServiceStub{
		ledgerEnd: 9,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testUtilitiesTransferOfferContract(
				"cbtc-offer",
				"BitSafe-validator-1::1220sender",
				"owner-party",
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
				"CBTC",
				"0.0010000000",
				executeBefore,
			)),
			activeContractResponse(testWalletOfferContract(
				"accepted-1",
				"AcceptedTransferOffer",
				"owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("5.0000000000"),
				"tracking-accepted",
				time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"): "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				switch req.URL.String() {
				case "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f/registry/metadata/v1/info":
					return httpJSONResponse(http.StatusOK, `{
						"adminId":"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
						"supportedApis":{}
					}`), nil
				case "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f/registry/metadata/v1/instruments/CBTC":
					return httpJSONResponse(http.StatusOK, `{
						"id":"CBTC",
						"name":"CBTC",
						"symbol":"CBTC",
						"decimals":8,
						"supportedApis":{}
					}`), nil
				default:
					return nil, fmt.Errorf("unexpected utilities registry URL %q", req.URL.String())
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(
		context.Background(),
		xclient.NewOfferArgs(xc.Address("owner-party"), xclient.OfferOptionContract(xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"))),
	)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "cbtc-offer", offers[0].ID)
	require.Equal(t, xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"), offers[0].AssetID)
	require.Equal(t, xc.Address("BitSafe-validator-1::1220sender"), offers[0].From)
	require.Equal(t, xc.Address("owner-party"), offers[0].To)
	require.Equal(t, "100000", offers[0].Amount.String())
	require.NotNil(t, offers[0].ExpiresAt)
	require.True(t, offers[0].ExpiresAt.Equal(executeBefore))
}

func TestListPendingOffersFallsBackToZeroAmountWhenDecimalsLookupFails(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 13,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletTransferRecordContract(
				"token-pending-no-decimals",
				"TransferOffer",
				"owner-party",
				"receiver-party",
				"issuer-party",
				"XC",
				"10.500000",
				"tracking-token",
				time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(context.Background(), xclient.NewOfferArgs(xc.Address("owner-party")))
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "token-pending-no-decimals", offers[0].ID)
	require.Equal(t, xc.ContractAddress("issuer-party#XC"), offers[0].AssetID)
	require.Equal(t, "0", offers[0].Amount.String())
}

func TestListSettlements(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 11,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletOfferContract(
				"pending-1",
				"TransferOffer",
				"owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("10.0000000000"),
				"tracking-pending",
				time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletOfferContract(
				"settlement-1",
				"AcceptedTransferOffer",
				"owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("3.2500000000"),
				"tracking-settlement",
				time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletOfferContract(
				"settlement-unrelated",
				"AcceptedTransferOffer",
				"other-sender",
				"other-receiver",
				cantonAmuletOfferAmount("7.0000000000"),
				"tracking-unrelated",
				time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	settlements, err := client.ListSettlements(context.Background(), xclient.NewOfferArgs(xc.Address("owner-party")))
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	require.Equal(t, "settlement-1", settlements[0].ID)
	require.Equal(t, xc.ContractAddress(xc.CANTON), settlements[0].AssetID)
	require.Equal(t, "32500000000", settlements[0].Amount.String())
	require.Equal(t, "tracking-settlement", settlements[0].TrackingID)
}

func TestListSettlementsFallsBackToZeroAmountWhenDecimalsLookupFails(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 14,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletTransferRecordContract(
				"settlement-no-decimals",
				"AcceptedTransferOffer",
				"owner-party",
				"receiver-party",
				"issuer-party",
				"XC",
				"3.250000",
				"tracking-settlement",
				time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
			)),
		},
	}

	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	settlements, err := client.ListSettlements(context.Background(), xclient.NewOfferArgs(xc.Address("owner-party")))
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	require.Equal(t, "settlement-no-decimals", settlements[0].ID)
	require.Equal(t, xc.ContractAddress("issuer-party#XC"), settlements[0].AssetID)
	require.Equal(t, "0", settlements[0].Amount.String())
}

func TestFetchCallInputOfferAcceptWallet(t *testing.T) {
	t.Parallel()

	sender := "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receiver := "receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction:  &interactive.PreparedTransaction{Transaction: &interactive.DamlTransaction{}},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletOfferContract(
				"wallet-offer-cid",
				"TransferOffer",
				sender,
				receiver,
				cantonAmuletOfferAmount("12.5"),
				"tracking-wallet",
				time.Unix(1710000000, 0).UTC(),
			)),
		},
	}
	interactiveStub := &interactiveSubmissionStub{prepareResp: prepareResp}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			interactiveSubmissionClient: interactiveStub,
			logger:                      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.OfferAcceptCall{ContractID: "wallet-offer-cid"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.OfferAccept, payload, xc.Address(receiver))
	require.NoError(t, err)

	inputI, err := client.FetchCallInput(context.Background(), callTx)
	require.NoError(t, err)
	input := inputI.(*tx_input.CallInput)
	require.Equal(t, prepareResp.GetPreparedTransaction(), input.PreparedTransaction)
	require.NotEmpty(t, input.SubmissionId)
	require.NotNil(t, interactiveStub.lastPrepareReq)

	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	require.Equal(t, "wallet-offer-cid", exercise.GetContractId())
	require.Equal(t, "TransferOffer_Accept", exercise.GetChoice())
	require.Equal(t, "Splice.Wallet.TransferOffer", exercise.GetTemplateId().GetModuleName())
	require.Equal(t, []string{receiver}, interactiveStub.lastPrepareReq.GetActAs())
	require.Equal(t, []string{receiver}, interactiveStub.lastPrepareReq.GetReadAs())
}

func TestFetchCallInputOfferAcceptUtilitiesTransferOffer(t *testing.T) {
	t.Parallel()

	sender := "BitSafe-validator-1::1220c6fc2e729dd1e4171702d2871841bb660a6d2ffa8d8b5bfe7415c9bc3d8cf362"
	receiver := "d6ed91f336502ff706d97729d7ab5521e230c39353ca79372d2b1fc239eaa72c::12203a20475db3ac28b1e0591c90de7826a205cacd9c7b724a2e5822851f029ee2fc"
	instrumentAdmin := "cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f"
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction:  &interactive.PreparedTransaction{Transaction: &interactive.DamlTransaction{}},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 456,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testUtilitiesTransferOfferContract(
				"utility-offer-cid",
				sender,
				receiver,
				instrumentAdmin,
				"CBTC",
				"0.0010000000",
				time.Unix(1710003600, 0).UTC(),
			)),
		},
	}
	interactiveStub := &interactiveSubmissionStub{prepareResp: prepareResp}
	packageStub := &packageManagementStub{
		resp: &admin.ListKnownPackagesResponse{
			PackageDetails: []*admin.PackageDetails{
				{Name: "splice-api-token-transfer-instruction-v1", PackageId: "transfer-package-id"},
			},
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"): "https://utilities.example/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			packageManagementClient:     packageStub,
			interactiveSubmissionClient: interactiveStub,
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "https://utilities.example/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f/registry/transfer-instruction/v1/utility-offer-cid/choice-contexts/accept", req.URL.String())
				body := `{
					"choiceContextData":{"values":{"note":{"tag":"AV_Text","value":"registry"}}},
					"disclosedContracts":[
						{
							"templateId":"#splice-api-token-transfer-instruction-v1:Splice.Api.Token.TransferInstructionV1:TransferFactory",
							"contractId":"factory-cid",
							"createdEventBlob":"AQ==",
							"synchronizerId":"sync-id"
						}
					]
				}`
				return httpJSONResponse(http.StatusOK, body), nil
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.OfferAcceptCall{ContractID: "utility-offer-cid"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.OfferAccept, payload, xc.Address(receiver))
	require.NoError(t, err)

	inputI, err := client.FetchCallInput(context.Background(), callTx)
	require.NoError(t, err)
	input := inputI.(*tx_input.CallInput)
	require.Equal(t, prepareResp.GetPreparedTransaction(), input.PreparedTransaction)
	require.NotNil(t, interactiveStub.lastPrepareReq)
	require.Equal(t, "sync-id", interactiveStub.lastPrepareReq.GetSynchronizerId())
	require.Len(t, interactiveStub.lastPrepareReq.GetDisclosedContracts(), 1)

	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	require.Equal(t, "utility-offer-cid", exercise.GetContractId())
	require.Equal(t, "TransferInstruction_Accept", exercise.GetChoice())
	require.Equal(t, "transfer-package-id", exercise.GetTemplateId().GetPackageId())
	require.Equal(t, "Splice.Api.Token.TransferInstructionV1", exercise.GetTemplateId().GetModuleName())
	extraArgsValue, ok := getRecordFieldValue(exercise.GetChoiceArgument().GetRecord(), "extraArgs")
	require.True(t, ok)
	contextValue, ok := getRecordFieldValue(extraArgsValue.GetRecord(), "context")
	require.True(t, ok)
	valuesField, ok := getRecordFieldValue(contextValue.GetRecord(), "values")
	require.True(t, ok)
	require.Len(t, valuesField.GetTextMap().GetEntries(), 1)
}

func TestFetchCallInputSettlementCompleteTargetsExactContract(t *testing.T) {
	t.Parallel()

	sender := "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receiver := "receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	prepareResp := &interactive.PrepareSubmissionResponse{
		PreparedTransaction:  &interactive.PreparedTransaction{Transaction: &interactive.DamlTransaction{}},
		HashingSchemeVersion: interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2,
	}
	stateStub := &stateServiceStub{
		ledgerEnd: 789,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletOfferContract(
				"settlement-1",
				"AcceptedTransferOffer",
				sender,
				receiver,
				cantonAmuletOfferAmount("1.0"),
				"tracking-1",
				time.Unix(1710000000, 0).UTC(),
			)),
			activeContractResponse(&v2.ActiveContract{
				CreatedEvent: testAmuletCreatedEvent(sender, "100.0"),
			}),
			activeContractResponse(testWalletOfferContract(
				"settlement-2",
				"AcceptedTransferOffer",
				sender,
				receiver,
				cantonAmuletOfferAmount("2.0"),
				"tracking-2",
				time.Unix(1710003600, 0).UTC(),
			)),
		},
	}
	interactiveStub := &interactiveSubmissionStub{prepareResp: prepareResp}
	now := time.Now().UTC()
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
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:                   "token",
			stateClient:                 stateStub,
			interactiveSubmissionClient: interactiveStub,
			validatorPartyID:            "validator-party",
			scanProxyURL:                "https://proxy.example",
			scanAPIURL:                  "https://scan.example",
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://proxy.example", req.URL.String())
				var envelope scanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				switch envelope.URL {
				case "https://scan.example/api/scan/v0/amulet-rules":
					body := `{
						"amulet_rules_update":{
							"contract":{
								"template_id":"rules-pkg:Splice.AmuletRules:AmuletRules",
								"contract_id":"rules-cid",
								"created_event_blob":"AQ==",
								"payload":{"dso":"dso-party"}
							},
							"domain_id":"sync-id"
						}
					}`
					return httpJSONResponse(http.StatusOK, body), nil
				case "https://scan.example/api/scan/v0/open-and-issuing-mining-rounds":
					body := fmt.Sprintf(`{
						"open_mining_rounds":{
							"1":{
								"contract":{
									"contract_id":"open-cid",
									"template_id":"round-pkg:Splice.Round:OpenMiningRound",
									"created_event_blob":"AQ==",
									"payload":{
										"round":{"number":"1"},
										"opensAt":%q,
										"targetClosesAt":%q
									}
								},
								"domain_id":"sync-id"
							}
						},
						"issuing_mining_rounds":{
							"1":{
								"contract":{
									"contract_id":"issuing-cid",
									"template_id":"round-pkg:Splice.Round:IssuingMiningRound",
									"created_event_blob":"AQ==",
									"payload":{
										"round":{"number":"1"},
										"opensAt":%q,
										"targetClosesAt":%q
									}
								},
								"domain_id":"sync-id"
							}
						}
					}`, now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(time.Hour).Format(time.RFC3339Nano), now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(time.Hour).Format(time.RFC3339Nano))
					return httpJSONResponse(http.StatusOK, body), nil
				default:
					return nil, fmt.Errorf("unexpected scan URL %q", envelope.URL)
				}
			})},
			logger: logrus.NewEntry(logrus.New()),
		},
		cantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		cantonUiUsername: "ui-user",
		cantonUiPassword: "ui-pass",
	}

	payload, err := json.Marshal(xccall.SettlementCompleteCall{ContractID: "settlement-2"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.SettlementComplete, payload, xc.Address(sender))
	require.NoError(t, err)

	inputI, err := client.FetchCallInput(context.Background(), callTx)
	require.NoError(t, err)
	input := inputI.(*tx_input.CallInput)
	require.Equal(t, prepareResp.GetPreparedTransaction(), input.PreparedTransaction)
	require.NotNil(t, interactiveStub.lastPrepareReq)
	require.Equal(t, "sync-id", interactiveStub.lastPrepareReq.GetSynchronizerId())

	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	require.Equal(t, "settlement-2", exercise.GetContractId())
	require.Equal(t, "AcceptedTransferOffer_Complete", exercise.GetChoice())
	require.Contains(t, interactiveStub.lastPrepareReq.GetReadAs(), sender)
	require.Contains(t, interactiveStub.lastPrepareReq.GetReadAs(), "validator-party")
}

func TestFetchCallInputReturnsUnsupportedTargetError(t *testing.T) {
	t.Parallel()

	party := "owner::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stateStub := &stateServiceStub{
		ledgerEnd: 1,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletOfferContract(
				"holding-cid",
				"SomethingElse",
				party,
				"receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				cantonAmuletOfferAmount("1.0"),
				"tracking",
				time.Unix(1710000000, 0).UTC(),
			)),
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:   "token",
			stateClient: stateStub,
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.OfferAcceptCall{ContractID: "holding-cid"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.OfferAccept, payload, xc.Address(party))
	require.NoError(t, err)

	_, err = client.FetchCallInput(context.Background(), callTx)
	require.ErrorContains(t, err, "unsupported offer accept target")
}

func TestFetchCallInputReturnsNotVisibleError(t *testing.T) {
	t.Parallel()

	party := "owner::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:   "token",
			stateClient: &stateServiceStub{ledgerEnd: 1},
			logger:      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.SettlementCompleteCall{ContractID: "missing-cid"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.SettlementComplete, payload, xc.Address(party))
	require.NoError(t, err)

	_, err = client.FetchCallInput(context.Background(), callTx)
	require.ErrorContains(t, err, "is not visible to caller")
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

	req := &interactive.ExecuteSubmissionAndWaitRequest{
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

	req := &interactive.ExecuteSubmissionAndWaitRequest{SubmissionId: "submission-id"}
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

func TestFetchTxInfoParsesTokenTransferInstructionAcceptMovement(t *testing.T) {
	t.Parallel()

	sender := "sender::1220dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	receiver := "receiver::1220eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	updateStub := &updateServiceStub{
		resp: &v2.GetUpdateResponse{
			Update: &v2.GetUpdateResponse_Transaction{
				Transaction: &v2.Transaction{
					UpdateId:       "update-token-accept",
					Offset:         105,
					SynchronizerId: "sync-id",
					EffectiveAt:    timestamppb.New(time.Unix(1700000000, 0)),
					Events: []*v2.Event{
						{
							Event: &v2.Event_Exercised{
								Exercised: &v2.ExercisedEvent{
									NodeId:               1,
									LastDescendantNodeId: 2,
									ContractId:           "transfer-instruction-cid",
									TemplateId: &v2.Identifier{
										ModuleName: "Splice.Api.Token.Test.SimpleTransferToken",
										EntityName: "SimpleTokenTransferInstruction",
									},
									InterfaceId: &v2.Identifier{
										PackageId:  "transfer-package-id",
										ModuleName: "Splice.Api.Token.TransferInstructionV1",
										EntityName: "TransferInstruction",
									},
									Choice:         "TransferInstruction_Accept",
									ChoiceArgument: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{}}},
									ActingParties:  []string{receiver},
									Consuming:      true,
									WitnessParties: []string{
										sender,
										receiver,
									},
								},
							},
						},
						{
							Event: &v2.Event_Created{
								Created: testTokenHoldingCreatedEventWithNodeID(
									"receiver-holding-cid",
									2,
									receiver,
									"issuer-party",
									"XC",
									"10.0",
								),
							},
						},
					},
				},
			},
		},
	}
	eventQueryStub := &eventQueryServiceStub{
		resp: &v2.GetEventsByContractIdResponse{
			Created: &v2.Created{
				CreatedEvent: testTokenTransferInstructionCreatedEvent(
					"transfer-instruction-cid",
					sender,
					receiver,
					"issuer-party",
					"XC",
					"10.0",
				),
			},
		},
	}
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 6,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		ledgerClient: &GrpcLedgerClient{
			authToken:        "token",
			stateClient:      &stateServiceStub{ledgerEnd: 110},
			updateClient:     updateStub,
			eventQueryClient: eventQueryStub,
			logger:           logrus.NewEntry(logrus.New()),
			packageManagementClient: &packageManagementStub{
				resp: &admin.ListKnownPackagesResponse{
					PackageDetails: []*admin.PackageDetails{
						{
							Name:      "splice-api-token-transfer-instruction-v1",
							PackageId: "transfer-package-id",
						},
					},
				},
			},
		},
	}

	owner, admin, instrumentID, amount, ok := extractTokenHoldingView(updateStub.resp.GetTransaction().GetEvents()[1].GetCreated())
	require.True(t, ok)
	require.Equal(t, receiver, owner)
	require.Equal(t, "issuer-party", admin)
	require.Equal(t, "XC", instrumentID)
	require.Equal(t, "10.0", amount)

	movements, err := client.extractTokenMovementsFromExercise(
		context.Background(),
		sender,
		updateStub.resp.GetTransaction().GetEvents()[0].GetExercised(),
		[]tokenHoldingCreation{{
			nodeID:          2,
			owner:           receiver,
			instrumentAdmin: "issuer-party",
			instrumentID:    "XC",
			amount:          "10.0",
		}},
		func(contract xc.ContractAddress) int32 { return 6 },
	)
	require.NoError(t, err)
	require.Len(t, movements, 1)

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-token-accept", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Len(t, info.Movements, 1)
	require.Equal(t, xc.ContractAddress("issuer-party#XC"), info.Movements[0].AssetId)
	require.Equal(t, xc.Address(sender), info.Movements[0].From[0].AddressId)
	require.Equal(t, xc.Address(receiver), info.Movements[0].To[0].AddressId)
	require.Equal(t, "10000000", info.Movements[0].From[0].Balance.String())
	require.NotNil(t, eventQueryStub.lastReq)
	require.Equal(t, "transfer-instruction-cid", eventQueryStub.lastReq.GetContractId())
	require.Contains(t, eventQueryStub.lastReq.GetEventFormat().GetFiltersByParty(), sender)
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
	}, "splice-wallet-package-id", "command-id", 1)

	create := cmd.GetCreate()
	require.NotNil(t, create)
	require.Equal(t, "splice-wallet-package-id", create.GetTemplateId().GetPackageId())
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				contract: "https://scan.example",
			},
		},
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				contract: "https://scan.example",
			},
		},
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
		cantonCfg: &CantonConfig{
			TokenRegistryURLs: map[xc.ContractAddress]string{
				contract: "https://scan.example",
			},
		},
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

type eventQueryServiceStub struct {
	lastReq *v2.GetEventsByContractIdRequest
	resp    *v2.GetEventsByContractIdResponse
	err     error
}

func (s *eventQueryServiceStub) GetEventsByContractId(_ context.Context, req *v2.GetEventsByContractIdRequest, _ ...grpc.CallOption) (*v2.GetEventsByContractIdResponse, error) {
	s.lastReq = req
	return s.resp, s.err
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

func activeContractResponse(contract *v2.ActiveContract) *v2.GetActiveContractsResponse {
	return &v2.GetActiveContractsResponse{
		ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
			ActiveContract: contract,
		},
	}
}

func testWalletOfferContract(contractID string, entityName string, sender string, receiver string, amountValue *v2.Value, trackingID string, expiresAt time.Time) *v2.ActiveContract {
	return &v2.ActiveContract{
		CreatedEvent: &v2.CreatedEvent{
			ContractId: contractID,
			TemplateId: &v2.Identifier{
				PackageId:  "wallet-package",
				ModuleName: "Splice.Wallet.TransferOffer",
				EntityName: entityName,
			},
			CreateArguments: &v2.Record{
				Fields: []*v2.RecordField{
					{Label: "sender", Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}}},
					{Label: "receiver", Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}}},
					{Label: "amount", Value: amountValue},
					{Label: "trackingId", Value: &v2.Value{Sum: &v2.Value_Text{Text: trackingID}}},
					{Label: "expiresAt", Value: &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: expiresAt.UnixMicro()}}},
				},
			},
		},
	}
}

func testWalletTransferRecordContract(contractID string, entityName string, sender string, receiver string, admin string, instrumentID string, amount string, trackingID string, expiresAt time.Time) *v2.ActiveContract {
	return &v2.ActiveContract{
		CreatedEvent: &v2.CreatedEvent{
			ContractId: contractID,
			TemplateId: &v2.Identifier{
				PackageId:  "wallet-package",
				ModuleName: "Splice.Wallet.TransferOffer",
				EntityName: entityName,
			},
			CreateArguments: &v2.Record{
				Fields: []*v2.RecordField{
					{
						Label: "transfer",
						Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
							Fields: []*v2.RecordField{
								{Label: "sender", Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}}},
								{Label: "receiver", Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}}},
								{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}}},
								{
									Label: "instrumentId",
									Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
										Fields: []*v2.RecordField{
											{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
											{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}}},
										},
									}}},
								},
							},
						}}},
					},
					{Label: "trackingId", Value: &v2.Value{Sum: &v2.Value_Text{Text: trackingID}}},
					{Label: "expiresAt", Value: &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: expiresAt.UnixMicro()}}},
				},
			},
		},
	}
}

func testUtilitiesTransferOfferContract(contractID string, sender string, receiver string, admin string, instrumentID string, amount string, executeBefore time.Time) *v2.ActiveContract {
	return &v2.ActiveContract{
		CreatedEvent: &v2.CreatedEvent{
			ContractId: contractID,
			TemplateId: &v2.Identifier{
				PackageId:  "utilities-package",
				ModuleName: "Utility.Registry.App.V0.Model.Transfer",
				EntityName: "TransferOffer",
			},
			CreateArguments: &v2.Record{
				Fields: []*v2.RecordField{
					{
						Label: "transfer",
						Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
							Fields: []*v2.RecordField{
								{Label: "sender", Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}}},
								{Label: "receiver", Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}}},
								{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}}},
								{
									Label: "instrumentId",
									Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
										Fields: []*v2.RecordField{
											{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
											{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}}},
										},
									}}},
								},
								{Label: "executeBefore", Value: &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: executeBefore.UnixMicro()}}},
							},
						}}},
					},
				},
			},
		},
	}
}

func cantonAmuletOfferAmount(amount string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
		Fields: []*v2.RecordField{
			{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}}},
			{Label: "unit", Value: &v2.Value{Sum: &v2.Value_Enum{Enum: &v2.Enum{Constructor: "AmuletUnit"}}}},
		},
	}}}
}

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
		CreateArguments: &v2.Record{
			Fields: []*v2.RecordField{
				{
					Label: "owner",
					Value: &v2.Value{Sum: &v2.Value_Party{Party: owner}},
				},
				{
					Label: "admin",
					Value: &v2.Value{Sum: &v2.Value_Party{Party: issuer}},
				},
				{
					Label: "symbol",
					Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}},
				},
				{
					Label: "amount",
					Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}},
				},
			},
		},
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

func testTokenHoldingCreatedEventWithNodeID(contractID string, nodeID int32, owner string, issuer string, instrumentID string, amount string) *v2.CreatedEvent {
	event := testTokenHoldingCreatedEventWithContractID(contractID, owner, issuer, instrumentID, amount)
	event.NodeId = nodeID
	event.InterfaceViews = nil
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

func testTokenTransferInstructionCreatedEvent(contractID string, sender string, receiver string, admin string, instrumentID string, amount string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: contractID,
		InterfaceViews: []*v2.InterfaceView{
			{
				InterfaceId: &v2.Identifier{
					PackageId:  "transfer-package-id",
					ModuleName: "Splice.Api.Token.TransferInstructionV1",
					EntityName: "TransferInstruction",
				},
				ViewValue: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "transfer",
							Value: &v2.Value{
								Sum: &v2.Value_Record{
									Record: &v2.Record{
										Fields: []*v2.RecordField{
											{Label: "sender", Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}}},
											{Label: "receiver", Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}}},
											{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: amount}}},
											{
												Label: "instrumentId",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
																{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: instrumentID}}},
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
