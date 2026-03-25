package client

import (
	"context"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
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

func TestSubmitTxRoutesCreateAccountPayloadToCreateAccountTxPath(t *testing.T) {
	t.Parallel()

	client := &Client{}
	err := client.SubmitTx(context.Background(), xctypes.SubmitTxReq{
		TxData: mustSerializedCreateAccountInput(t),
	})
	require.ErrorContains(t, err, "create-account transaction is not signed")
}

func TestSubmitTxExecutesStandardCantonPayload(t *testing.T) {
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

	err = client.SubmitTx(context.Background(), xctypes.SubmitTxReq{TxData: payload})
	require.NoError(t, err)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "submission-id", stub.lastReq.GetSubmissionId())
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

type interactiveSubmissionStub struct {
	lastReq *interactive.ExecuteSubmissionAndWaitRequest
}

func (s *interactiveSubmissionStub) PrepareSubmission(context.Context, *interactive.PrepareSubmissionRequest, ...grpc.CallOption) (*interactive.PrepareSubmissionResponse, error) {
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
