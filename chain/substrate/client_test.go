package substrate

import (
	_ "embed"

	xc "github.com/cordialsys/crosschain"
	xclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
)

// *** RPC & HTTP Test Responses ***
// A copy of the metadata JSON blob from RPC call. This is SCALE-encoded hex that decodes to Substrate Metadata
//
//go:embed test_rpc_meta.json
var test_rpc_meta string

// A copy of the response from account balance RPC call. SCALE-encoded hex bytes.
//
//go:embed test_rpc_storage.json
var test_rpc_storage string

// A copy of a response from subscan's API query on transaction/extrinsic
//
//go:embed test_http_tx.json
var test_http_tx string

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(&xc.ChainConfig{})
	require.NotNil(err)
	require.Nil(client.DotClient)
}

func (s *CrosschainTestSuite) TestBalance() {
	require := s.Require()

	// Note that these RPC calls do have IDs, which the client library increments per call but doesn't care if they
	// don't match up. The test json files all use ID 1.
	rpc, rpcClose := testtypes.MockJSONRPC(s.T(), []string{test_rpc_meta, test_rpc_meta, test_rpc_storage})
	defer rpcClose()

	client, err := NewClient(&xc.ChainConfig{
		Chain:    "DOT",
		Driver:   "substrate",
		URL:      rpc.URL,
		Decimals: 10,
		ChainID:  0,
	})
	require.Nil(err)
	require.NotNil(client.DotClient)

	res, err := client.FetchBalance(s.Ctx, "1598AR2pgoJCWHn3UA2FTemJ74hBWgp7GLyNB4oSkt6vqMno")
	require.Nil(err)
	require.Equal(uint64(132919986950892), res.Uint64())
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()

	http, httpClose := testtypes.MockHTTP(s.T(), test_http_tx, 200)
	defer httpClose()

	client, err := NewClient(&xc.ChainConfig{
		Chain:       "DOT",
		Driver:      "substrate",
		Decimals:    10,
		ChainID:     0,
		ExplorerURL: http.URL,
	})
	require.Equal(CheckError(err), xclient.NetworkError) // Intentionally missing RPC
	require.NotNil(client.Asset)

	res, err := client.FetchLegacyTxInfo(s.Ctx, "47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb")
	require.Nil(err)
	correct := xc.LegacyTxInfo{
		BlockHash:  "0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4",
		TxID:       "16097417-2",
		From:       xc.Address("138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
		To:         xc.Address("12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"),
		Amount:     xc.NewAmountBlockchainFromUint64(872321233400),
		Fee:        xc.NewAmountBlockchainFromUint64(157316518),
		BlockIndex: 16097417,
		BlockTime:  1687547412,
	}
	require.Equal(correct, res)
}

func (s *CrosschainTestSuite) TestFetchTxInfoFail() {
	require := s.Require()

	http, httpClose := testtypes.MockHTTP(s.T(), `{"code":0,"message":"Success","generated_at":1688400923,"data":null}`, 200)
	defer httpClose()

	client, err := NewClient(&xc.ChainConfig{
		Chain:       "DOT",
		Driver:      "substrate",
		Decimals:    10,
		ChainID:     0,
		ExplorerURL: http.URL,
	})
	require.Equal(CheckError(err), xclient.NetworkError)
	require.NotNil(client.Asset)

	// Nonexistent hash
	_, err = client.FetchLegacyTxInfo(s.Ctx, "47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c02811")
	require.Equal(CheckError(err), xclient.NetworkError)
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	// TODO: write test
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	// TODO: write test
}

// TODO: write a list of RPC responses that correspond to a whole transaction sequence (metadata, storage,... hash)
// Use subsequences for testing

func (s *CrosschainTestSuite) TestEstimateGas() {
	require := s.Require()

	// TODO: convert to Mock
	rpc, rpcClose := testtypes.MockJSONRPC(s.T(), []string{
		test_rpc_meta,
		test_rpc_meta,
		// chain_getBlockHash
		`"0x91b171bb158e2d3848fa23a9f1c25182fb8e20313b2c1eb49219da7a70ce90c3"`,
		// state_getRuntimeVersion
		`{"specName":"polkadot","implName":"parity-polkadot","authoringVersion":0,"specVersion":9420,"implVersion":0,"apis":[["0xdf6acb689907609b",4],["0x37e397fc7c91f5e4",2],["0x40fe3ad401f8959a",6],["0x17a6bc0d0062aeb3",1],["0x18ef58a3b67ba770",1],["0xd2bc9897eed08f15",3],["0xf78b278be53f454c",2],["0xaf2c0297a23e6d3d",4],["0x49eaaf1b548a0cb0",2],["0x91d5df18b0d2cf58",2],["0xed99c5acb25eedf5",3],["0xcbca25e39f142387",2],["0x687ad44ad37f03c2",1],["0xab3c0572291feb8b",1],["0xbc9d89904f5b923f",1],["0x37c8bb1350a9a2a8",4],["0xf3ff14d5ab527059",3]],"transactionVersion":23,"stateVersion":0}`,
		// chain_getHeader
		`{"parentHash":"0xc62eee473fa0dc73e6dceb29ee1c6727ce1aa963cac41c8d3a96d07f1d63cd4a","number":"0xf8b92a","stateRoot":"0x458cbe8b4af2db75b74b89d8f32f643106cd690b35301c0e11f9cd65fa5a282c","extrinsicsRoot":"0x5b3b7a3f8f8052ab5b6dfbddebea8f4867e552022d8ac658b6a787817385424d","digest":{"logs":["0x0642414245b501030301000016c1c6100000000004c2d1ceda540b84e0de2e2278567218dbd56346e5131f9761259f9e7235347772243d462bb690ad46342a4b6a8c161fed9742b06340cb4ff611447e2d9e9e0867eff979498f14f2bbf8dbff6dad55ab5898dc9efb934e4733b199ef5c90ee0f","0x0542414245010130054b72a125f167cb6371cfc93b46121ce5e73779c11da97f099ec745aff273bd48488185d8382cb4b96dfd101ed2c8f792fcced772d6136048989022deb88d"]}}`,
		// chain_getBlockHash
		`"0x229ef9cc262f4df162cb2dae39a0540d8fe231b14094c5a47518c3de2a010c06"`,
		//state_getStorage,
		"null",
		// payment_queryFeeDetails
		`{"inclusionFee":{"baseFee":"0x989680","lenFee":"0x8677d40","adjustedWeightFee":"0x1416a6"}}`,
	})
	defer rpcClose()

	client, err := NewClient(&xc.ChainConfig{
		Chain:    "DOT",
		Driver:   "substrate",
		URL:      rpc.URL,
		Decimals: 10,
		ChainID:  0,
	})
	require.Nil(err)
	amt, err := client.EstimateGas(s.Ctx)
	require.Nil(err)
	println(amt.Uint64())
	require.EqualValues(amt.Uint64(), 1080258)
}
