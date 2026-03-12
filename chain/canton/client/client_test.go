package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	xccall "github.com/cordialsys/crosschain/call"
	cantoncall "github.com/cordialsys/crosschain/chain/canton/call"
	. "github.com/cordialsys/crosschain/chain/canton/client"
	cantonkc "github.com/cordialsys/crosschain/chain/canton/keycloak"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	v2 "github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/admin"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	xclient "github.com/cordialsys/crosschain/client"
	cantonclientconfig "github.com/cordialsys/crosschain/client/canton"
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
			CantonConfig: &CantonConfig{
				KeycloakRealm: "xyz",
			},
		}
		_, err := NewClient(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing canton config field")
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC": "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			ScanProxyURL: "https://proxy.example",
			ScanAPIURL:   "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "https://proxy.example", req.URL.String())
				require.Equal(t, "Bearer scan-token", req.Header.Get("Authorization"))

				var envelope ScanProxyRequest
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("DUMMY", "", "issuer-party#DummyHolding", 10, xc.AmountHumanReadable{}),
	}

	decimals, err := client.FetchDecimals(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, 18, decimals)

	decimals, err = client.FetchDecimals(context.Background(), xc.ContractAddress(xc.CANTON))
	require.NoError(t, err)
	require.Equal(t, 18, decimals)

	decimals, err = client.FetchDecimals(context.Background(), "issuer-party#DummyHolding")
	require.NoError(t, err)
	require.Equal(t, 10, decimals)

	decimals, err = client.FetchDecimals(context.Background(), "issuer-party#Unconfigured")
	require.NoError(t, err)
	require.Equal(t, 6, decimals)

	_, err = client.FetchDecimals(context.Background(), "SOME_TOKEN")
	require.ErrorContains(t, err, "invalid Canton token contract")
}

func TestFetchBlock(t *testing.T) {
	t.Parallel()

	newClient := func(ledgerEnd int64) *Client {
		return &Client{
			Asset: &xc.ChainConfig{
				ChainBaseConfig: &xc.ChainBaseConfig{
					Chain:  xc.CANTON,
					Driver: xc.DriverCanton,
				},
				ChainClientConfig: &xc.ChainClientConfig{},
			},
			LedgerClient: &GrpcLedgerClient{
				StateClient: &stateServiceStub{ledgerEnd: ledgerEnd},
				Logger:      logrus.NewEntry(logrus.New()),
			},
		}
	}

	t.Run("latest uses ledger end", func(t *testing.T) {
		block, err := newClient(123).FetchBlock(context.Background(), xclient.LatestHeight())
		require.NoError(t, err)
		require.Equal(t, "123", block.Height.String())
		require.Equal(t, "ledger-offset/123", block.Hash)
		require.Empty(t, block.TransactionIds)
	})

	t.Run("specific height uses requested offset", func(t *testing.T) {
		block, err := newClient(123).FetchBlock(context.Background(), xclient.AtHeight(100))
		require.NoError(t, err)
		require.Equal(t, "100", block.Height.String())
		require.Equal(t, "ledger-offset/100", block.Hash)
		require.Empty(t, block.TransactionIds)
	})

	t.Run("future height errors", func(t *testing.T) {
		_, err := newClient(123).FetchBlock(context.Background(), xclient.AtHeight(124))
		require.Error(t, err)
		require.Contains(t, err.Error(), "after current ledger end")
	})
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC": "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			ScanProxyURL: "https://proxy.example",
			ScanAPIURL:   "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var envelope ScanProxyRequest
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
	}

	_, err := client.FetchDecimals(context.Background(), "issuer-party#Missing")
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"issuer-party#XC": "https://registry.example",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			ScanProxyURL: "https://proxy.example",
			ScanAPIURL:   "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "https://registry.example/registry/metadata/v1/info", req.URL.String())
				return httpJSONResponse(http.StatusOK, `{
						"adminId":"other-admin",
						"supportedApis":{}
					}`), nil
			})},
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
	}

	_, err := client.FetchDecimals(context.Background(), "issuer-party#XC")
	require.ErrorContains(t, err, `registry admin "other-admin" does not match instrument admin "issuer-party"`)
}

func TestFetchBalanceTokenHolding(t *testing.T) {
	t.Parallel()

	party := xc.Address("Owner-party")
	stateStub := &stateServiceStub{
		ledgerEnd: 123,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("Owner-party", "issuer-party", "DummyHolding", "12.3456789012"),
					},
				},
			},
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("Owner-party", "issuer-party", "OtherToken", "99.0"),
					},
				},
			},
			{
				ContractEntry: &v2.GetActiveContractsResponse_ActiveContract{
					ActiveContract: &v2.ActiveContract{
						CreatedEvent: testTokenHoldingCreatedEvent("other-Owner", "issuer-party", "DummyHolding", "88.0"),
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"issuer-party#DummyHolding": "https://registry.example",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			AuthToken:               "token",
			StateClient:             stateStub,
			PackageManagementClient: packageStub,
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Logger: logrus.NewEntry(logrus.New()),
		},
	}

	balance, err := client.FetchBalance(context.Background(), xclient.NewBalanceArgs(party, xclient.BalanceOptionContract("issuer-party#DummyHolding")))
	require.NoError(t, err)
	require.Equal(t, "12345678", balance.String())
	require.NotNil(t, stateStub.lastActiveContractsReq)
	require.Contains(t, stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty(), "Owner-party")
	filter := stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty()["Owner-party"].GetCumulative()[0].GetInterfaceFilter()
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC": "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Logger: logrus.NewEntry(logrus.New()),
		},
	}

	decimals, err := client.FetchDecimals(context.Background(), "cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC")
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
				"Owner-party",
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
				"Owner-party",
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
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(context.Background(), xclient.NewOfferArgs(xc.Address("Owner-party")))
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "pending-1", offers[0].ID)
	require.Equal(t, xc.ContractAddress(xc.CANTON), offers[0].AssetID)
	require.Equal(t, xc.Address("Owner-party"), offers[0].From)
	require.Equal(t, xc.Address("receiver-party"), offers[0].To)
	require.Equal(t, "100000000000", offers[0].Amount.String())
	require.Equal(t, "tracking-1", offers[0].TrackingID)
	require.NotNil(t, offers[0].ExpiresAt)
	require.NotNil(t, stateStub.lastActiveContractsReq)
	require.Contains(t, stateStub.lastActiveContractsReq.GetEventFormat().GetFiltersByParty(), "Owner-party")
}

func TestListPendingOffersFiltersByTokenContract(t *testing.T) {
	t.Parallel()

	stateStub := &stateServiceStub{
		ledgerEnd: 7,
		activeContractsResponses: []*v2.GetActiveContractsResponse{
			activeContractResponse(testWalletTransferRecordContract(
				"token-pending",
				"TransferOffer",
				"Owner-party",
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
				"Owner-party",
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
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}
	client.Asset.NativeAssets = []*xc.AdditionalNativeAsset{
		xc.NewAdditionalNativeAsset("XC", "", "issuer-party#XC", 6, xc.AmountHumanReadable{}),
		xc.NewAdditionalNativeAsset("OTHER", "", "issuer-party#OTHER", 6, xc.AmountHumanReadable{}),
	}

	offers, err := client.ListPendingOffers(
		context.Background(),
		xclient.NewOfferArgs(xc.Address("Owner-party"), xclient.OfferOptionContract("issuer-party#XC")),
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
				"Owner-party",
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
				"CBTC",
				"0.0010000000",
				executeBefore,
			)),
			activeContractResponse(testWalletOfferContract(
				"accepted-1",
				"AcceptedTransferOffer",
				"Owner-party",
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC": "https://api.utilities.digitalasset-staging.com/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Logger: logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(
		context.Background(),
		xclient.NewOfferArgs(xc.Address("Owner-party"), xclient.OfferOptionContract("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC")),
	)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.Equal(t, "cbtc-offer", offers[0].ID)
	require.Equal(t, xc.ContractAddress("cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC"), offers[0].AssetID)
	require.Equal(t, xc.Address("BitSafe-validator-1::1220sender"), offers[0].From)
	require.Equal(t, xc.Address("Owner-party"), offers[0].To)
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
				"Owner-party",
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
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	offers, err := client.ListPendingOffers(context.Background(), xclient.NewOfferArgs(xc.Address("Owner-party")))
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
				"Owner-party",
				"receiver-party",
				cantonAmuletOfferAmount("10.0000000000"),
				"tracking-pending",
				time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			)),
			activeContractResponse(testWalletOfferContract(
				"settlement-1",
				"AcceptedTransferOffer",
				"Owner-party",
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
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	settlements, err := client.ListSettlements(context.Background(), xclient.NewOfferArgs(xc.Address("Owner-party")))
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
				"Owner-party",
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
		LedgerClient: &GrpcLedgerClient{
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	settlements, err := client.ListSettlements(context.Background(), xclient.NewOfferArgs(xc.Address("Owner-party")))
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			InteractiveSubmissionClient: interactiveStub,
			Logger:                      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: "wallet-offer-cid"})
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
	InstrumentAdmin := "cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f"
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
				InstrumentAdmin,
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
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				"cbtc-network::12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f#CBTC": "https://utilities.example/api/token-standard/v0/registrars/cbtc-network%3A%3A12201b1741b63e2494e4214cf0bedc3d5a224da53b3bf4d76dba468f8e97eb15508f",
			},
		},
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			PackageManagementClient:     packageStub,
			InteractiveSubmissionClient: interactiveStub,
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Logger: logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: "utility-offer-cid"})
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
	extraArgsValue, ok := GetRecordFieldValue(exercise.GetChoiceArgument().GetRecord(), "extraArgs")
	require.True(t, ok)
	contextValue, ok := GetRecordFieldValue(extraArgsValue.GetRecord(), "context")
	require.True(t, ok)
	valuesField, ok := GetRecordFieldValue(contextValue.GetRecord(), "values")
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			InteractiveSubmissionClient: interactiveStub,
			ValidatorPartyID:            "validator-party",
			ScanProxyURL:                "https://proxy.example",
			ScanAPIURL:                  "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://proxy.example", req.URL.String())
				var envelope ScanProxyRequest
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
	}

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: "settlement-2"})
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

	party := "Owner::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:   "token",
			StateClient: stateStub,
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: "holding-cid"})
	require.NoError(t, err)
	callTx, err := cantoncall.NewCall(client.Asset.Base(), xccall.OfferAccept, payload, xc.Address(party))
	require.NoError(t, err)

	_, err = client.FetchCallInput(context.Background(), callTx)
	require.ErrorContains(t, err, "unsupported offer accept target")
}

func TestFetchCallInputReturnsNotVisibleError(t *testing.T) {
	t.Parallel()

	party := "Owner::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig:   &xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		LedgerClient: &GrpcLedgerClient{
			AuthToken:   "token",
			StateClient: &stateServiceStub{ledgerEnd: 1},
			Logger:      logrus.NewEntry(logrus.New()),
		},
	}

	payload, err := json.Marshal(xccall.SomeContractCall{ContractID: "missing-cid"})
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

	userID, err := ValidatorServiceUserIDFromToken(header + "." + payload + ".sig")
	require.NoError(t, err)
	require.Equal(t, "service-account-validator-id", userID)
}

func TestSubmitTxRequiresMetadata(t *testing.T) {
	t.Parallel()

	stub := &interactiveSubmissionStub{}
	client := &Client{
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			InteractiveSubmissionClient: stub,
			Logger:                      logrus.NewEntry(logrus.New()),
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			InteractiveSubmissionClient: stub,
			Logger:                      logrus.NewEntry(logrus.New()),
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

func TestSubmitTxLogsPaidTrafficCost(t *testing.T) {
	var logs strings.Builder
	logger := logrus.New()
	logger.SetOutput(&logs)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	stub := &interactiveSubmissionStub{
		executeResp: &interactive.ExecuteSubmissionAndWaitResponse{
			UpdateId:         "update-id",
			CompletionOffset: 12,
		},
	}
	completionStub := &completionServiceStub{
		responses: []*v2.CompletionStreamResponse{
			{
				CompletionResponse: &v2.CompletionStreamResponse_Completion{
					Completion: &v2.Completion{
						SubmissionId:    "submission-id",
						UpdateId:        "update-id",
						Offset:          12,
						PaidTrafficCost: 12345,
					},
				},
			},
		},
	}
	client := &Client{
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			InteractiveSubmissionClient: stub,
			CompletionClient:            completionStub,
			ValidatorServiceUserID:      "validator-user",
			Logger:                      logrus.NewEntry(logger),
		},
	}

	req := &interactive.ExecuteSubmissionAndWaitRequest{
		SubmissionId: "submission-id",
		PartySignatures: &interactive.PartySignatures{
			Signatures: []*interactive.SinglePartySignatures{{Party: "sender-party"}},
		},
	}
	payload, err := proto.Marshal(req)
	require.NoError(t, err)
	metadata, err := cantontx.NewTransferMetadata().Bytes()
	require.NoError(t, err)

	err = client.SubmitTx(context.Background(), xctypes.SubmitTxReq{
		TxData:         payload,
		BroadcastInput: string(metadata),
	})
	require.NoError(t, err)
	require.Contains(t, logs.String(), `"paid_traffic_cost":12345`)
	require.Contains(t, logs.String(), `"msg":"canton submission completed"`)
	require.Equal(t, int64(11), completionStub.lastReq.GetBeginExclusive())
	require.Equal(t, []string{"sender-party"}, completionStub.lastReq.GetParties())
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:              "token",
			StateClient:            &stateServiceStub{ledgerEnd: 110},
			UpdateClient:           updateStub,
			CompletionClient:       completionStub,
			ValidatorServiceUserID: "service-account-validator-id",
			Logger:                 logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("100-submission-id", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Equal(t, "100-submission-id", info.Hash)
	require.Equal(t, "update-123", info.LookupId)
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
		LedgerClient: &GrpcLedgerClient{
			Logger: logrus.NewEntry(logrus.New()),
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:    "token",
			StateClient:  &stateServiceStub{ledgerEnd: 110},
			UpdateClient: updateStub,
			Logger:       logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-123", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Equal(t, "update-123", info.Hash)
	require.Equal(t, "update-123", info.LookupId)
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

func TestFetchTxInfoDirectUpdateLookupResolvesSenderWithLighthouse(t *testing.T) {
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:        "token",
			StateClient:      &stateServiceStub{ledgerEnd: 110},
			UpdateClient:     updateStub,
			LighthouseAPIURL: "https://lighthouse.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://lighthouse.example/api/transactions/update-123", req.URL.String())
				body := fmt.Sprintf(`{"transaction":{"update_id":"update-123"},"events":{"verdict":{"submitting_parties":[%q]}}}`, sender)
				return httpJSONResponse(http.StatusOK, body), nil
			})},
			Logger: logrus.NewEntry(logrus.New()),
		},
	}

	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-123"))
	require.NoError(t, err)
	require.Equal(t, "update-123", info.Hash)
	require.Equal(t, "update-123", info.LookupId)
	require.Len(t, info.Movements, 2)
	require.NotNil(t, updateStub.lastReq)
	require.Nil(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersForAnyParty())
	require.Contains(t, updateStub.lastReq.GetUpdateFormat().GetIncludeTransactions().GetEventFormat().GetFiltersByParty(), sender)
	require.Equal(t, xc.Address(sender), info.Movements[0].From[0].AddressId)
	require.Equal(t, xc.Address(receiver), info.Movements[0].To[0].AddressId)
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:    "token",
			StateClient:  &stateServiceStub{ledgerEnd: 110},
			UpdateClient: updateStub,
			Logger:       logrus.NewEntry(logrus.New()),
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:        "token",
			StateClient:      &stateServiceStub{ledgerEnd: 110},
			UpdateClient:     updateStub,
			EventQueryClient: eventQueryStub,
			Logger:           logrus.NewEntry(logrus.New()),
			PackageManagementClient: &packageManagementStub{
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

	Owner, admin, InstrumentID, Amount, ok := ExtractTokenHoldingView(updateStub.resp.GetTransaction().GetEvents()[1].GetCreated())
	require.True(t, ok)
	require.Equal(t, receiver, Owner)
	require.Equal(t, "issuer-party", admin)
	require.Equal(t, "XC", InstrumentID)
	require.Equal(t, "10.0", Amount)

	movements, err := client.ExtractTokenMovementsFromExercise(
		context.Background(),
		sender,
		updateStub.resp.GetTransaction().GetEvents()[0].GetExercised(),
		[]TokenHoldingCreation{{
			NodeID:          2,
			Owner:           receiver,
			InstrumentAdmin: "issuer-party",
			InstrumentID:    "XC",
			Amount:          "10.0",
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
	require.Equal(t, "instruction memo", info.Movements[0].Memo)
	require.NotNil(t, eventQueryStub.lastReq)
	require.Equal(t, "transfer-instruction-cid", eventQueryStub.lastReq.GetContractId())
	require.Contains(t, eventQueryStub.lastReq.GetEventFormat().GetFiltersByParty(), sender)
}

func TestFetchTxInfoParsesTokenTransferFactoryMovement(t *testing.T) {
	t.Parallel()

	sender := "sender::1220ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	receiver := "receiver::1220111111111111111111111111111111111111111111111111111111111111"
	dso := "DSO::1220222222222222222222222222222222222222222222222222222222222222"
	updateStub := &updateServiceStub{
		resp: &v2.GetUpdateResponse{
			Update: &v2.GetUpdateResponse_Transaction{
				Transaction: &v2.Transaction{
					UpdateId:       "update-token-factory",
					Offset:         106,
					SynchronizerId: "sync-id",
					EffectiveAt:    timestamppb.New(time.Unix(1700000100, 0)),
					Events: []*v2.Event{
						{
							Event: &v2.Event_Exercised{
								Exercised: testTokenTransferFactoryExerciseEvent(
									sender,
									receiver,
									dso,
									"Amulet",
									"0.1000000000",
									"factory memo",
								),
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
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{
				Confirmations: xc.ConfirmationsConfig{Final: 1},
			},
		},
		LedgerClient: &GrpcLedgerClient{
			AuthToken:    "token",
			StateClient:  &stateServiceStub{ledgerEnd: 110},
			UpdateClient: updateStub,
			Logger:       logrus.NewEntry(logrus.New()),
		},
	}
	info, err := client.FetchTxInfo(context.Background(), txinfo.NewArgs("update-token-factory", txinfo.OptionSender(xc.Address(sender))))
	require.NoError(t, err)
	require.Len(t, info.Movements, 1)
	require.Equal(t, xc.ContractAddress(xc.CANTON), info.Movements[0].AssetId)
	require.Equal(t, xc.ContractAddress(xc.CANTON), info.Movements[0].XContract)
	require.Equal(t, xc.Address(sender), info.Movements[0].From[0].AddressId)
	require.Equal(t, xc.Address(receiver), info.Movements[0].To[0].AddressId)
	require.Equal(t, "1000000000", info.Movements[0].From[0].Balance.String())
	require.Equal(t, "factory memo", info.Movements[0].Memo)
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

	fee, ok := ExtractTransferFee(ex, 18)
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

	got, ok := ExtractTransferSender(ex)
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
		xcbuilder.OptionMemo("native memo"),
	)
	require.NoError(t, err)

	cmd := BuildTransferOfferCreateCommand(args, AmuletRules{
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
	descriptionValue, ok := GetRecordFieldValue(create.GetCreateArguments(), "description")
	require.True(t, ok)
	require.Equal(t, "native memo", descriptionValue.GetText())
}

func TestBuildTransferPreapprovalExerciseCommandUsesArgsAmount(t *testing.T) {
	t.Parallel()

	args, err := xcbuilder.NewTransferArgs(
		&xc.ChainBaseConfig{Chain: xc.CANTON, Driver: xc.DriverCanton},
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
		xcbuilder.OptionMemo("preapproval memo"),
	)
	require.NoError(t, err)

	cmd, _, err := BuildTransferPreapprovalExerciseCommand(
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
	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	require.Equal(t, "12.3", extractCommandAmountNumeric(t, choiceArgument))
	descriptionValue, ok := GetRecordFieldValue(choiceArgument, "description")
	require.True(t, ok)
	require.Equal(t, "preapproval memo", descriptionValue.GetOptional().GetValue().GetText())
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
		xcbuilder.OptionMemo("token memo"),
	)
	require.NoError(t, err)

	cmd, err := BuildTokenStandardTransferCommand(
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
	require.Equal(t, TokenTransferModule, exercise.GetTemplateId().GetModuleName())
	require.Equal(t, TokenTransferEntity, exercise.GetTemplateId().GetEntityName())
	require.Equal(t, "factory-cid", exercise.GetContractId())
	require.Equal(t, "TransferFactory_Transfer", exercise.GetChoice())

	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	require.NotNil(t, choiceArgument)
	transferValue, ok := GetRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	require.NotNil(t, transferValue.GetRecord())
	require.Equal(t, "12.3", extractCommandAmountNumeric(t, transferValue.GetRecord()))
	requestedAtValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "requestedAt")
	require.True(t, ok)
	require.Equal(t, time.Unix(1700000000, 123000000).UTC().UnixMicro(), requestedAtValue.GetTimestamp())
	executeBeforeValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "executeBefore")
	require.True(t, ok)
	require.Equal(t, time.Unix(1700086400, 456000000).UTC().UnixMicro(), executeBeforeValue.GetTimestamp())
	requireTokenMetadataMemo(t, transferValue.GetRecord(), "token memo")

	instrumentValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "instrumentId")
	require.True(t, ok)
	require.Equal(t, "issuer-party", instrumentValue.GetRecord().GetFields()[0].GetValue().GetParty())
	require.Equal(t, "XC", instrumentValue.GetRecord().GetFields()[1].GetValue().GetText())

	inputHoldingValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "inputHoldingCids")
	require.True(t, ok)
	require.Len(t, inputHoldingValue.GetList().GetElements(), 1)
	require.Equal(t, "holding-cid", inputHoldingValue.GetList().GetElements()[0].GetContractId())

	extraArgsValue, ok := GetRecordFieldValue(choiceArgument, "extraArgs")
	require.True(t, ok)
	requireTokenMetadataEmpty(t, extraArgsValue.GetRecord())
	contextValue, ok := GetRecordFieldValue(extraArgsValue.GetRecord(), "context")
	require.True(t, ok)
	valuesField, ok := GetRecordFieldValue(contextValue.GetRecord(), "values")
	require.True(t, ok)
	require.Len(t, valuesField.GetTextMap().GetEntries(), 1)
}

func TestBuildTokenStandardTransferCommandSkipsUnexpiredLockedHoldings(t *testing.T) {
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

	requestedAt := time.Unix(1700000000, 0).UTC()
	cmd, err := BuildTokenStandardTransferCommand(
		args,
		"transfer-package-id",
		"factory-cid",
		map[string]any{"values": map[string]any{}},
		[]*v2.ActiveContract{
			{CreatedEvent: testTokenHoldingCreatedEventWithContractID("unlocked-cid", string(args.GetFrom()), "issuer-party", "XC", "100.0")},
			{CreatedEvent: testLockedTokenHoldingCreatedEventWithContractID("locked-future-cid", string(args.GetFrom()), "issuer-party", "XC", "100.0", requestedAt.Add(time.Hour))},
			{CreatedEvent: testLockedTokenHoldingCreatedEventWithContractID("locked-expired-cid", string(args.GetFrom()), "issuer-party", "XC", "100.0", requestedAt.Add(-time.Hour))},
		},
		1,
		requestedAt,
		requestedAt.Add(24*time.Hour),
	)
	require.NoError(t, err)

	choiceArgument := cmd.GetExercise().GetChoiceArgument().GetRecord()
	transferValue, ok := GetRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	inputHoldingValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "inputHoldingCids")
	require.True(t, ok)
	require.Len(t, inputHoldingValue.GetList().GetElements(), 2)
	require.Equal(t, "unlocked-cid", inputHoldingValue.GetList().GetElements()[0].GetContractId())
	require.Equal(t, "locked-expired-cid", inputHoldingValue.GetList().GetElements()[1].GetContractId())
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			PackageManagementClient:     packageStub,
			InteractiveSubmissionClient: interactiveStub,
			ScanProxyURL:                "https://proxy.example",
			ScanAPIURL:                  "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://proxy.example", req.URL.String())
				require.Equal(t, "Bearer scan-token", req.Header.Get("Authorization"))

				var envelope ScanProxyRequest
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				cantonclientconfig.TokenRegistryKey(contract): "https://scan.example",
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
		xcbuilder.OptionMemo("registry memo"),
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
	requireRegistryMetadataMemo(t, registryTransfer, "registry memo")
	registryExtraArgs, ok := registryChoiceArgs["extraArgs"].(map[string]any)
	require.True(t, ok)
	requireRegistryMetadataEmpty(t, registryExtraArgs)
	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	transferValue, ok := GetRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	requireTokenMetadataMemo(t, transferValue.GetRecord(), "registry memo")
	extraArgsValue, ok := GetRecordFieldValue(choiceArgument, "extraArgs")
	require.True(t, ok)
	requireTokenMetadataEmpty(t, extraArgsValue.GetRecord())
	requestedAtValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "requestedAt")
	require.True(t, ok)
	require.Equal(t, registryTransfer["requestedAt"], time.UnixMicro(requestedAtValue.GetTimestamp()).UTC().Format(time.RFC3339Nano))
	executeBeforeValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "executeBefore")
	require.True(t, ok)
	require.Equal(t, registryTransfer["executeBefore"], time.UnixMicro(executeBeforeValue.GetTimestamp()).UTC().Format(time.RFC3339Nano))
}

func TestFetchTransferInputNativeUsesTokenStandardTransferFactory(t *testing.T) {
	t.Parallel()

	dso := "DSO::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
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
						CreatedEvent: testTokenHoldingCreatedEventWithContractID("holding-cid", "sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dso, "Amulet", "100.0"),
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

	var registryChoiceArgs map[string]any
	client := &Client{
		Asset: &xc.ChainConfig{
			ChainBaseConfig: &xc.ChainBaseConfig{
				Chain:    xc.CANTON,
				Driver:   xc.DriverCanton,
				Decimals: 10,
			},
			ChainClientConfig: &xc.ChainClientConfig{},
		},
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			PackageManagementClient:     packageStub,
			InteractiveSubmissionClient: interactiveStub,
			ScanProxyURL:                "https://proxy.example",
			ScanAPIURL:                  "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://proxy.example", req.URL.String())
				require.Equal(t, "Bearer scan-token", req.Header.Get("Authorization"))

				var envelope ScanProxyRequest
				require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
				switch envelope.URL {
				case "https://scan.example/api/scan/v0/amulet-rules":
					require.Equal(t, http.MethodPost, envelope.Method)
					return httpJSONResponse(http.StatusOK, `{
						"amulet_rules_update":{
							"contract":{
								"template_id":"rules-pkg:Splice.AmuletRules:AmuletRules",
								"contract_id":"amulet-rules-cid",
								"created_event_blob":"AQ==",
								"payload":{"dso":"`+dso+`"}
							},
							"domain_id":"domain"
						}
					}`), nil
				case "https://scan.example/registry/transfer-instruction/v1/transfer-factory":
					require.Equal(t, http.MethodPost, envelope.Method)
					var requestBody map[string]any
					require.NoError(t, json.Unmarshal([]byte(envelope.Body), &requestBody))
					var ok bool
					registryChoiceArgs, ok = requestBody["choiceArguments"].(map[string]any)
					require.True(t, ok)
					return httpJSONResponse(http.StatusOK, `{
						"factoryId":"factory-cid",
						"transferKind":"self",
						"choiceContext":{
							"choiceContextData":{
								"values":{
									"amulet-rules":{"tag":"AV_ContractId","value":"amulet-rules-cid"}
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
					}`), nil
				default:
					return nil, fmt.Errorf("unexpected scan proxy target %q", envelope.URL)
				}
			})},
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
		CantonCfg:        &CantonConfig{},
	}

	args, err := xcbuilder.NewTransferArgs(
		client.Asset.GetChain().Base(),
		xc.Address("sender::1220aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		xc.Address("receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
		xc.NewAmountBlockchainFromUint64(123),
		xcbuilder.OptionMemo("native memo"),
	)
	require.NoError(t, err)

	inputI, err := client.FetchTransferInput(context.Background(), args)
	require.NoError(t, err)

	input, ok := inputI.(*tx_input.TxInput)
	require.True(t, ok)
	require.Equal(t, xc.ContractAddress(dso+"#Amulet"), input.ContractAddress)
	require.Equal(t, int32(10), input.Decimals)
	require.Equal(t, prepareResp.GetPreparedTransaction(), input.PreparedTransaction)
	require.NotNil(t, registryChoiceArgs)
	registryTransfer, ok := registryChoiceArgs["transfer"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, dso, registryChoiceArgs["expectedAdmin"])
	requireRegistryMetadataMemo(t, registryTransfer, "native memo")
	registryInstrument, ok := registryTransfer["instrumentId"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, dso, registryInstrument["admin"])
	require.Equal(t, "Amulet", registryInstrument["id"])
	require.Equal(t, []string{string(args.GetFrom())}, interactiveStub.lastPrepareReq.GetActAs())
	exercise := interactiveStub.lastPrepareReq.GetCommands()[0].GetExercise()
	require.Equal(t, "factory-cid", exercise.GetContractId())
	require.Equal(t, "TransferFactory_Transfer", exercise.GetChoice())
	choiceArgument := exercise.GetChoiceArgument().GetRecord()
	expectedAdminValue, ok := GetRecordFieldValue(choiceArgument, "expectedAdmin")
	require.True(t, ok)
	require.Equal(t, dso, expectedAdminValue.GetParty())
	transferValue, ok := GetRecordFieldValue(choiceArgument, "transfer")
	require.True(t, ok)
	requireTokenMetadataMemo(t, transferValue.GetRecord(), "native memo")
	instrumentValue, ok := GetRecordFieldValue(transferValue.GetRecord(), "instrumentId")
	require.True(t, ok)
	require.Equal(t, dso, instrumentValue.GetRecord().GetFields()[0].GetValue().GetParty())
	require.Equal(t, "Amulet", instrumentValue.GetRecord().GetFields()[1].GetValue().GetText())
	extraArgsValue, ok := GetRecordFieldValue(choiceArgument, "extraArgs")
	require.True(t, ok)
	requireTokenMetadataEmpty(t, extraArgsValue.GetRecord())
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			PackageManagementClient:     packageStub,
			InteractiveSubmissionClient: interactiveStub,
			ScanProxyURL:                "https://proxy.example",
			ScanAPIURL:                  "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var envelope ScanProxyRequest
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				cantonclientconfig.TokenRegistryKey(contract): "https://scan.example",
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
	transferValue, ok := GetRecordFieldValue(choiceArgument, "transfer")
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
		LedgerClient: &GrpcLedgerClient{
			AuthToken:                   "token",
			StateClient:                 stateStub,
			PackageManagementClient:     packageStub,
			InteractiveSubmissionClient: interactiveStub,
			ScanProxyURL:                "https://proxy.example",
			ScanAPIURL:                  "https://scan.example",
			HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Logger: logrus.NewEntry(logrus.New()),
		},
		CantonUiKC:       cantonkc.NewClient(keycloakServer.URL, "test", "client", "secret", "validator-party"),
		CantonUiUsername: "ui-user",
		CantonUiPassword: "ui-pass",
		CantonCfg: &CantonConfig{
			TokenRegistryURLs: map[cantonclientconfig.TokenRegistryKey]string{
				cantonclientconfig.TokenRegistryKey(contract): "https://scan.example",
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
		ScanProxyURL: "https://proxy.example",
		ScanAPIURL:   "https://scan.example",
		HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://proxy.example", req.URL.String())
			require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
			require.Equal(t, "application/json", req.Header.Get("Content-Type"))

			var envelope ScanProxyRequest
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
		ScanProxyURL: "https://proxy.example",
		ScanAPIURL:   "https://scan.example/",
		HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var envelope ScanProxyRequest
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

func TestGetTransferPreapprovalByPartyUsesScanProxyRequest(t *testing.T) {
	party := "receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	client := &GrpcLedgerClient{
		ScanProxyURL: "https://proxy.example",
		ScanAPIURL:   "https://scan.example/",
		HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var envelope ScanProxyRequest
			require.NoError(t, json.NewDecoder(req.Body).Decode(&envelope))
			require.Equal(t, "GET", envelope.Method)
			require.Equal(t, "https://scan.example/api/scan/v0/transfer-preapprovals/by-party/"+url.PathEscape(party), envelope.URL)
			require.Equal(t, "application/json", envelope.Headers["Content-Type"])

			body := `{"transfer_preapproval":{"contract":{"contract_id":"preapproval-cid","template_id":"pkg:Splice.AmuletRules:TransferPreapproval","created_event_blob":"AQ==","payload":{"dso":"dso-party","receiver":"receiver::1220bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","provider":"provider-party"}},"domain_id":"domain"}}`
			return httpJSONResponse(http.StatusOK, body), nil
		})},
	}

	contract, synchronizerID, err := client.GetTransferPreapprovalByParty(context.Background(), "token", party)
	require.NoError(t, err)
	require.Equal(t, "domain", synchronizerID)
	created := contract.GetCreatedEvent()
	require.NotNil(t, created)
	require.Equal(t, "preapproval-cid", created.GetContractId())
	require.Equal(t, "pkg", created.GetTemplateId().GetPackageId())
	require.Equal(t, "Splice.AmuletRules", created.GetTemplateId().GetModuleName())
	require.Equal(t, "TransferPreapproval", created.GetTemplateId().GetEntityName())
	require.Equal(t, []byte{0x01}, created.GetCreatedEventBlob())
	receiverValue, ok := GetRecordFieldValue(created.GetCreateArguments(), "receiver")
	require.True(t, ok)
	require.Equal(t, party, receiverValue.GetParty())
}

func TestGetTransferPreapprovalByPartyReturnsNilForNotFound(t *testing.T) {
	client := &GrpcLedgerClient{
		ScanProxyURL: "https://proxy.example",
		ScanAPIURL:   "https://scan.example/",
		HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return httpJSONResponse(http.StatusNotFound, `{"error":"missing"}`), nil
		})},
	}

	contract, synchronizerID, err := client.GetTransferPreapprovalByParty(context.Background(), "token", "missing-party")
	require.NoError(t, err)
	require.Nil(t, contract)
	require.Empty(t, synchronizerID)
}

func TestGetAmuletRulesPreservesScanProxyHTTPError(t *testing.T) {
	client := &GrpcLedgerClient{
		ScanProxyURL: "https://proxy.example",
		ScanAPIURL:   "https://scan.example",
		HttpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
	executeResp      *interactive.ExecuteSubmissionAndWaitResponse
	executeErr       error
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
	if s.executeResp != nil || s.executeErr != nil {
		return s.executeResp, s.executeErr
	}
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

func testWalletTransferRecordContract(contractID string, entityName string, sender string, receiver string, admin string, InstrumentID string, Amount string, trackingID string, expiresAt time.Time) *v2.ActiveContract {
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
								{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}}},
								{
									Label: "instrumentId",
									Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
										Fields: []*v2.RecordField{
											{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
											{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}}},
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

func testUtilitiesTransferOfferContract(contractID string, sender string, receiver string, admin string, InstrumentID string, Amount string, executeBefore time.Time) *v2.ActiveContract {
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
								{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}}},
								{
									Label: "instrumentId",
									Value: &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
										Fields: []*v2.RecordField{
											{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
											{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}}},
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

func cantonAmuletOfferAmount(Amount string) *v2.Value {
	return &v2.Value{Sum: &v2.Value_Record{Record: &v2.Record{
		Fields: []*v2.RecordField{
			{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}}},
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

func testAmuletCreatedEvent(Owner string, initialAmount string) *v2.CreatedEvent {
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
						Sum: &v2.Value_Party{Party: Owner},
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

func testTokenHoldingCreatedEvent(Owner string, issuer string, InstrumentID string, Amount string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: "token-contract-id",
		CreateArguments: &v2.Record{
			Fields: []*v2.RecordField{
				{
					Label: "owner",
					Value: &v2.Value{Sum: &v2.Value_Party{Party: Owner}},
				},
				{
					Label: "admin",
					Value: &v2.Value{Sum: &v2.Value_Party{Party: issuer}},
				},
				{
					Label: "symbol",
					Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}},
				},
				{
					Label: "amount",
					Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}},
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
							Value: &v2.Value{Sum: &v2.Value_Party{Party: Owner}},
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
												Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}},
											},
										},
									},
								},
							},
						},
						{
							Label: "amount",
							Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}},
						},
					},
				},
			},
		},
	}
}

func testTokenHoldingCreatedEventWithContractID(contractID string, Owner string, issuer string, InstrumentID string, Amount string) *v2.CreatedEvent {
	event := testTokenHoldingCreatedEvent(Owner, issuer, InstrumentID, Amount)
	event.ContractId = contractID
	return event
}

func testLockedTokenHoldingCreatedEventWithContractID(contractID string, Owner string, issuer string, InstrumentID string, Amount string, expiresAt time.Time) *v2.CreatedEvent {
	event := testTokenHoldingCreatedEventWithContractID(contractID, Owner, issuer, InstrumentID, Amount)
	event.TemplateId = &v2.Identifier{
		PackageId:  "amulet-package-id",
		ModuleName: "Splice.Amulet",
		EntityName: EntityLockedAmulet,
	}
	event.CreateArguments.Fields = append(event.CreateArguments.Fields, &v2.RecordField{
		Label: "lock",
		Value: &v2.Value{
			Sum: &v2.Value_Record{
				Record: &v2.Record{
					Fields: []*v2.RecordField{
						{
							Label: "expiresAt",
							Value: &v2.Value{Sum: &v2.Value_Timestamp{Timestamp: expiresAt.UnixMicro()}},
						},
					},
				},
			},
		},
	})
	return event
}

func testTokenHoldingCreatedEventWithNodeID(contractID string, NodeID int32, Owner string, issuer string, InstrumentID string, Amount string) *v2.CreatedEvent {
	event := testTokenHoldingCreatedEventWithContractID(contractID, Owner, issuer, InstrumentID, Amount)
	event.NodeId = NodeID
	event.InterfaceViews = nil
	return event
}

func testTokenTransferFactoryCreatedEvent(contractID string, admin string, InstrumentID string) *v2.CreatedEvent {
	return &v2.CreatedEvent{
		ContractId: contractID,
		CreateArguments: &v2.Record{
			Fields: []*v2.RecordField{
				{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
				{Label: "symbol", Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}}},
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

func testTokenTransferInstructionCreatedEvent(contractID string, sender string, receiver string, admin string, InstrumentID string, Amount string) *v2.CreatedEvent {
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
											{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}}},
											{
												Label: "instrumentId",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
																{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}}},
															},
														},
													},
												},
											},
											{
												Label: "meta",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{
																	Label: "values",
																	Value: &v2.Value{
																		Sum: &v2.Value_TextMap{
																			TextMap: &v2.TextMap{
																				Entries: []*v2.TextMap_Entry{
																					{
																						Key:   "splice.lfdecentralizedtrust.org/reason",
																						Value: &v2.Value{Sum: &v2.Value_Text{Text: "instruction memo"}},
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
		},
	}
}

func testTokenTransferFactoryExerciseEvent(sender string, receiver string, admin string, InstrumentID string, Amount string, memo string) *v2.ExercisedEvent {
	return &v2.ExercisedEvent{
		NodeId:               1,
		LastDescendantNodeId: 1,
		ContractId:           "factory-cid",
		InterfaceId: &v2.Identifier{
			PackageId:  "transfer-package-id",
			ModuleName: "Splice.Api.Token.TransferInstructionV1",
			EntityName: "TransferFactory",
		},
		Choice:        "TransferFactory_Transfer",
		ActingParties: []string{sender},
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
											{Label: "sender", Value: &v2.Value{Sum: &v2.Value_Party{Party: sender}}},
											{Label: "receiver", Value: &v2.Value{Sum: &v2.Value_Party{Party: receiver}}},
											{Label: "amount", Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}}},
											{
												Label: "instrumentId",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{Label: "admin", Value: &v2.Value{Sum: &v2.Value_Party{Party: admin}}},
																{Label: "id", Value: &v2.Value{Sum: &v2.Value_Text{Text: InstrumentID}}},
															},
														},
													},
												},
											},
											{
												Label: "meta",
												Value: &v2.Value{
													Sum: &v2.Value_Record{
														Record: &v2.Record{
															Fields: []*v2.RecordField{
																{
																	Label: "values",
																	Value: &v2.Value{
																		Sum: &v2.Value_TextMap{
																			TextMap: &v2.TextMap{
																				Entries: []*v2.TextMap_Entry{
																					{
																						Key:   "splice.lfdecentralizedtrust.org/reason",
																						Value: &v2.Value{Sum: &v2.Value_Text{Text: memo}},
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
	t.Fatalf("Amount field not found")
	return ""
}

func requireTokenMetadataMemo(t *testing.T, record *v2.Record, memo string) {
	t.Helper()

	metaValue, ok := GetRecordFieldValue(record, "meta")
	require.True(t, ok)
	valuesValue, ok := GetRecordFieldValue(metaValue.GetRecord(), "values")
	require.True(t, ok)
	entries := valuesValue.GetTextMap().GetEntries()
	require.Len(t, entries, 1)
	require.Equal(t, "splice.lfdecentralizedtrust.org/reason", entries[0].GetKey())
	require.Equal(t, memo, entries[0].GetValue().GetText())
}

func requireTokenMetadataEmpty(t *testing.T, record *v2.Record) {
	t.Helper()

	metaValue, ok := GetRecordFieldValue(record, "meta")
	require.True(t, ok)
	valuesValue, ok := GetRecordFieldValue(metaValue.GetRecord(), "values")
	require.True(t, ok)
	require.Empty(t, valuesValue.GetTextMap().GetEntries())
}

func requireRegistryMetadataMemo(t *testing.T, record map[string]any, memo string) {
	t.Helper()

	metaValue, ok := record["meta"].(map[string]any)
	require.True(t, ok)
	valuesValue, ok := metaValue["values"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, memo, valuesValue["splice.lfdecentralizedtrust.org/reason"])
}

func requireRegistryMetadataEmpty(t *testing.T, record map[string]any) {
	t.Helper()

	metaValue, ok := record["meta"].(map[string]any)
	require.True(t, ok)
	valuesValue, ok := metaValue["values"].(map[string]any)
	require.True(t, ok)
	require.Empty(t, valuesValue)
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

func testAmuletRulesTransferEvent(sender string, receiver string, Amount string) *v2.ExercisedEvent {
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
																					Value: &v2.Value{Sum: &v2.Value_Numeric{Numeric: Amount}},
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
