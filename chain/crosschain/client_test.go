package crosschain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	"github.com/cordialsys/crosschain/chain/evm"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx   context.Context
	Asset xc.ITask
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
	s.Asset = &xc.AssetConfig{
		Clients: []*xc.ClientConfig{
			{},
		},
	}
}

func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

// Client

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(s.Asset)
	require.NotNil(client)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()

	txInput := evm.NewTxInput()
	txInput.Nonce = 1234567
	resObj := types.TxInputRes{
		TxInputReq: &types.TxInputReq{},
		TxInput:    txInput,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(&s.Suite, string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	input, err := client.FetchTxInput(s.Ctx, from, to, xc.AmountBlockchain{})
	require.Nil(err)
	require.IsType(txInput, input)
	require.Equal(txInput, input)
}

func (s *CrosschainTestSuite) TestFetchTxInputError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	_, err := client.FetchTxInput(s.Ctx, from, to, xc.AmountBlockchain{})
	require.EqualError(err, "api-error")
}

func (s *CrosschainTestSuite) TestFetchTxInputErrorFallback() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	server2, close2 := testtypes.MockJSONRPC(&s.Suite, errors.New(`{"message": "custom RPC error", "code": 123}`))
	defer close2()

	s.Asset.GetNativeAsset().Driver = string(xc.DriverSolana)
	s.Asset.GetNativeAsset().URL = server2.URL
	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	from := xc.Address("from")
	to := xc.Address("to")
	_, err := client.FetchTxInput(s.Ctx, from, to, xc.AmountBlockchain{})
	require.ErrorContains(err, "custom RPC error")
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()

	txInfo := xc.TxInfo{
		BlockHash:     "block-hash",
		BlockIndex:    2,
		TxID:          "tx-hash",
		Confirmations: 10,
	}
	resObj := types.TxInfoRes{
		TxInfoReq: &types.TxInfoReq{},
		TxInfo:    txInfo,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(&s.Suite, string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	txHash := xc.TxHash("hash")
	info, err := client.FetchTxInfo(s.Ctx, txHash)
	require.Nil(err)
	require.IsType(txInfo, info)
	require.Equal(txInfo, info)
}

func (s *CrosschainTestSuite) TestFetchTxInfoError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	txHash := xc.TxHash("hash")
	_, err := client.FetchTxInfo(s.Ctx, txHash)
	require.EqualError(err, "api-error")
}

func (s *CrosschainTestSuite) TestFetchTxInfoErrorFallback() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	server2, close2 := testtypes.MockJSONRPC(&s.Suite, errors.New(`{"message": "custom RPC error", "code": 123}`))
	defer close2()

	s.Asset.GetNativeAsset().Driver = string(xc.DriverSolana)
	s.Asset.GetNativeAsset().URL = server2.URL
	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	// note: need a valid tx hash because go-solana checks
	txHash := xc.TxHash("5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw")
	_, err := client.FetchTxInfo(s.Ctx, txHash)
	require.ErrorContains(err, "custom RPC error")
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	require := s.Require()

	resObj := types.SubmitTxRes{
		SubmitTxReq: &types.SubmitTxReq{},
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(&s.Suite, string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := &types.SubmitTxReq{
		TxData: []byte("data"),
	}
	err := client.SubmitTx(s.Ctx, txData)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestSubmitTxError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := &types.SubmitTxReq{
		TxData: []byte("data"),
	}
	err := client.SubmitTx(s.Ctx, txData)
	require.EqualError(err, "api-error")
}

func (s *CrosschainTestSuite) TestSubmitTxErrorFallback() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	server2, close2 := testtypes.MockJSONRPC(&s.Suite, errors.New(`{"message": "custom RPC error", "code": 123}`))
	defer close2()

	s.Asset.GetNativeAsset().Driver = string(xc.DriverSolana)
	s.Asset.GetNativeAsset().URL = server2.URL
	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	// types.SubmitTxReq implements xc.Tx so it's easy to use here
	txData := &types.SubmitTxReq{
		TxData: []byte("data"),
	}
	err := client.SubmitTx(s.Ctx, txData)
	require.ErrorContains(err, "custom RPC error")
}

func (s *CrosschainTestSuite) TestFetchBalance() {
	require := s.Require()

	expectedBalance := xc.NewAmountBlockchainFromUint64(1234567)
	resObj := types.BalanceRes{
		BalanceReq: &types.BalanceReq{},
		BalanceRaw: expectedBalance,
	}
	res, _ := json.Marshal(resObj)

	server, close := testtypes.MockHTTP(&s.Suite, string(res), 200)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	address := xc.Address("address")
	balance, err := client.FetchBalance(s.Ctx, address)
	require.Nil(err)
	require.Equal(expectedBalance, balance)

	balance, err = client.FetchNativeBalance(s.Ctx, address)
	require.Nil(err)
	require.Equal(expectedBalance, balance)
}

func (s *CrosschainTestSuite) TestFetchBalanceError() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	address := xc.Address("address")
	_, err := client.FetchBalance(s.Ctx, address)
	require.EqualError(err, "api-error")

	_, err = client.FetchNativeBalance(s.Ctx, address)
	require.EqualError(err, "api-error")
}

func (s *CrosschainTestSuite) TestFetchBalanceErrorFallback() {
	require := s.Require()

	server, close := testtypes.MockHTTP(&s.Suite, `{"code":3,"message":"api-error"}`, 400)
	defer close()

	server2, close2 := testtypes.MockJSONRPC(&s.Suite, errors.New(`{"message": "custom RPC error", "code": 123}`))
	defer close2()

	s.Asset.GetNativeAsset().Driver = string(xc.DriverSolana)
	s.Asset.GetNativeAsset().URL = server2.URL
	client, _ := NewClient(s.Asset)
	client.URL = server.URL

	// note: need a valid address because go-solana checks
	address := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
	_, err := client.FetchBalance(s.Ctx, address)
	require.ErrorContains(err, "custom RPC error")

	_, err = client.FetchNativeBalance(s.Ctx, address)
	require.ErrorContains(err, "custom RPC error")
}
