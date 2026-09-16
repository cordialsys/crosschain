package crosschain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx   context.Context
	Asset *xc.ChainConfig
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
	s.Asset = xc.NewChainConfig("")
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(s.Asset, "", "", "", 0)
	require.NotNil(client)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()

	txInput := evminput.NewTxInput()
	txInput.Nonce = 1234567
	txInputBz, _ := json.Marshal(txInput)
	resObj := types.LegacyTxInputRes{
		TransferInputReq: &types.TransferInputReq{},
		NewTxInput:       txInputBz,
		TxInput:          txInput,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchLegacyTxInput(s.Ctx, from, to)
	require.NoError(err)
	require.IsType(txInput, input)
	require.Equal(txInput, input)
}

func TestFetchTransferInputFeeContract(t *testing.T) {
	const contract = xc.ContractAddress("0x20c0000000000000000000000000000000000001")
	const feeContract = xc.ContractAddress("0x20c0000000000000000000000000000000000000")
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%t", explicit), func(t *testing.T) {
			requests := make(chan map[string]json.RawMessage, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- request
				input, err := json.Marshal(evminput.NewTxInput())
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if err := json.NewEncoder(w).Encode(types.LegacyTxInputRes{NewTxInput: input}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			chain := xc.NewChainConfig(xc.TEMPO)
			client, err := NewClient(chain, server.URL, "", "", 0)
			require.NoError(t, err)
			opts := []builder.BuilderOption{builder.OptionContractAddress(contract)}
			if explicit {
				opts = append(opts, builder.OptionFeeContract(feeContract))
			}
			args, err := builder.NewTransferArgs(chain.Base(), "from", "to", xc.NewAmountBlockchainFromUint64(1), opts...)
			require.NoError(t, err)
			_, err = client.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			request := <-requests
			require.JSONEq(t, fmt.Sprintf("%q", contract), string(request["contract"]))
			if explicit {
				require.JSONEq(t, fmt.Sprintf("%q", feeContract), string(request["fee_contract"]))
			} else {
				require.NotContains(t, request, "fee_contract")
			}
		})
	}
}

func (s *CrosschainTestSuite) TestFetchTxInputError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(
		s.T(),
		fmt.Sprintf(`{"code":%d,"message":"api-error"}`, codes.FailedPrecondition),
		400,
	)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	_, err := client.FetchLegacyTxInput(s.Ctx, from, to)
	require.EqualError(err, "FailedPrecondition: api-error")

	errNative, ok := err.(*errors.Error)
	require.True(ok, "client did not map error to native error")
	require.Equal(errors.FailedPrecondition, errNative.Status)
	require.Equal("api-error", errNative.Message)
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()

	txInfo := txinfo.LegacyTxInfo{
		BlockHash:     "block-hash",
		BlockIndex:    2,
		TxID:          "tx-hash",
		Confirmations: 10,
	}
	resObj := types.TxLegacyInfoRes{
		TxInfoReq:    &types.TxInfoReq{},
		LegacyTxInfo: txInfo,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	txHash := xc.TxHash("hash")
	info, err := client.FetchLegacyTxInfo(s.Ctx, txHash)
	require.Nil(err)
	require.IsType(txInfo, info)
	require.Equal(txInfo, info)
}

func (s *CrosschainTestSuite) TestFetchTxInfoError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(s.T(), `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	txHash := xc.TxHash("hash")
	_, err := client.FetchLegacyTxInfo(s.Ctx, txHash)
	require.EqualError(err, "InvalidArgument: api-error")
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	require := s.Require()

	resObj := types.SubmitTxRes{
		SubmitTxReq: &xctypes.SubmitTxReq{},
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := xctypes.SubmitTxReq{
		TxData:             []byte("data"),
		LegacyTxSignatures: [][]byte{{1, 2, 3, 4}},
		BroadcastInput:     `{"x":"y"}`,
	}
	err := client.SubmitTx(s.Ctx, txData, builder.SubmitArgs{})
	require.NoError(err)
}

func (s *CrosschainTestSuite) TestSubmitTxError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(s.T(), `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := xctypes.SubmitTxReq{
		TxData: []byte("data"),
	}
	err := client.SubmitTx(s.Ctx, txData, builder.SubmitArgs{})
	require.EqualError(err, "InvalidArgument: api-error")
}

func (s *CrosschainTestSuite) TestFetchBalance() {
	require := s.Require()

	expectedBalance := xc.NewAmountBlockchainFromUint64(1234567)
	resObj := types.BalanceRes{
		BalanceReq:  &types.BalanceReq{},
		XBalanceRaw: expectedBalance,
		Balance:     expectedBalance,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	address := xc.Address("address")
	balanceArgs := xclient.NewBalanceArgs(address)
	balance, err := client.FetchBalance(s.Ctx, balanceArgs)
	require.Nil(err)
	require.Equal(expectedBalance, balance)

	balance, err = client.FetchNativeBalance(s.Ctx, address)
	require.Nil(err)
	require.Equal(expectedBalance, balance)
}

func (s *CrosschainTestSuite) TestFetchBalanceError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(s.T(), `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	address := xc.Address("address")
	balanceArgs := xclient.NewBalanceArgs(address)
	_, err := client.FetchBalance(s.Ctx, balanceArgs)
	require.EqualError(err, "InvalidArgument: api-error")

	_, err = client.FetchNativeBalance(s.Ctx, address)
	require.EqualError(err, "InvalidArgument: api-error")
}

func (s *CrosschainTestSuite) TestGetAccountState() {
	require := s.Require()

	resObj := types.AccountStateRes{
		State: xclient.AccountInactive,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	args := xclient.NewCreateAccountArgs(xc.Address("address"), []byte{0x01, 0x02})
	state, err := client.GetAccountState(s.Ctx, args)
	require.NoError(err)
	require.NotEmpty(state)
	require.Equal(xclient.AccountInactive, state)
}

func (s *CrosschainTestSuite) TestGetAccountStateError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(s.T(), `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "", 0)
	client.URL = server.URL

	args := xclient.NewCreateAccountArgs(xc.Address("address"), []byte{0x01, 0x02})
	_, err := client.GetAccountState(s.Ctx, args)
	require.EqualError(err, "InvalidArgument: api-error")
}
