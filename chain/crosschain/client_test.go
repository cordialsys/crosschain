package crosschain

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx   context.Context
	Asset xc.ITask
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
	client, err := NewClient(s.Asset, "", "", "")
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

	client, _ := NewClient(s.Asset, "", "", "")
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchLegacyTxInput(s.Ctx, from, to)
	require.NoError(err)
	require.IsType(txInput, input)
	require.Equal(txInput, input)
}

func (s *CrosschainTestSuite) TestFetchTxInputError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(
		s.T(),
		fmt.Sprintf(`{"code":%d,"message":"api-error"}`, codes.FailedPrecondition),
		400,
	)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "")
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

	txInfo := xclient.LegacyTxInfo{
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

	client, _ := NewClient(s.Asset, "", "", "")
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

	client, _ := NewClient(s.Asset, "", "", "")
	client.URL = server.URL

	txHash := xc.TxHash("hash")
	_, err := client.FetchLegacyTxInfo(s.Ctx, txHash)
	require.EqualError(err, "InvalidArgument: api-error")
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	require := s.Require()

	resObj := types.SubmitTxRes{
		SubmitTxReq: &types.SubmitTxReq{},
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(s.T(), string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "")
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := &types.SubmitTxReq{
		TxData:       []byte("data"),
		TxSignatures: [][]byte{{1, 2, 3, 4}},
	}
	err := client.SubmitTx(s.Ctx, txData)
	require.NoError(err)
}

func (s *CrosschainTestSuite) TestSubmitTxError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(s.T(), `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset, "", "", "")
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := &types.SubmitTxReq{
		TxData: []byte("data"),
	}
	err := client.SubmitTx(s.Ctx, txData)
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

	client, _ := NewClient(s.Asset, "", "", "")
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

	client, _ := NewClient(s.Asset, "", "", "")
	client.URL = server.URL

	address := xc.Address("address")
	balanceArgs := xclient.NewBalanceArgs(address)
	_, err := client.FetchBalance(s.Ctx, balanceArgs)
	require.EqualError(err, "InvalidArgument: api-error")

	_, err = client.FetchNativeBalance(s.Ctx, address)
	require.EqualError(err, "InvalidArgument: api-error")
}
