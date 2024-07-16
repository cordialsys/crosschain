package substrate_test

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
	xclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
)

// reserialize will drop internal fields set by constructors
func reserialize(tx *xclient.TxInfo) *xclient.TxInfo {
	bz, _ := json.Marshal(tx)
	var info xclient.TxInfo
	json.Unmarshal(bz, &info)
	return &info
}

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
	_, err := substrate.NewClient(&xc.ChainConfig{})
	require.Error(err)
}

func (s *CrosschainTestSuite) TestBalance() {
	require := s.Require()

	// Note that these RPC calls do have IDs, which the client library increments per call but doesn't care if they
	// don't match up. The test json files all use ID 1.
	rpc, rpcClose := testtypes.MockJSONRPC(s.T(), []string{test_rpc_meta, test_rpc_meta, test_rpc_storage})
	defer rpcClose()

	client, err := substrate.NewClient(&xc.ChainConfig{
		Chain:      "DOT",
		Driver:     "substrate",
		URL:        rpc.URL,
		IndexerUrl: "subscan",
		AuthSecret: "aaa",
		Decimals:   10,
		ChainID:    0,
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

	client, err := substrate.NewTxInfoClient(&xc.ChainConfig{
		Chain:       "DOT",
		Driver:      "substrate",
		URL:         http.URL,
		IndexerUrl:  http.URL,
		AuthSecret:  "aaa",
		ChainPrefix: "0",
		Decimals:    10,
		ChainID:     0,
		ExplorerURL: http.URL,
	})
	require.NoError(err)
	require.NotNil(client)

	res, err := client.FetchTxInfo(s.Ctx, "47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb")
	require.NoError(err)

	// xclient.TxInfo{
	// 	BlockHash:  "0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4",
	// 	TxID:       "16097417-2",
	// 	From:       xc.Address("138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
	// 	To:         xc.Address("12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"),
	// 	Amount:     xc.NewAmountBlockchainFromUint64(872321233400),
	// 	Fee:        xc.NewAmountBlockchainFromUint64(157316518),
	// 	BlockIndex: 16097417,
	// 	BlockTime:  1687547412,
	// }
	correct := &xclient.TxInfo{
		Name:  "chains/DOT/transactions/0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
		Hash:  "0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
		Chain: xc.DOT,
		Block: &xclient.Block{
			Height: 16097417,
			Hash:   "0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4",
			Time:   time.Unix(1687547412, 0),
		},
		Transfers: []*xclient.Transfer{
			{
				From: []*xclient.BalanceChange{
					{
						Asset:    "chains/DOT/assets/DOT",
						Contract: "DOT",
						Balance:  xc.NewAmountBlockchainFromUint64(872321233400),
						Address:  xclient.NewAddressName(xc.DOT, "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
					},
				},
				To: []*xclient.BalanceChange{
					{
						Asset:    "chains/DOT/assets/DOT",
						Contract: "DOT",
						Balance:  xc.NewAmountBlockchainFromUint64(872321233400),
						Address:  xclient.NewAddressName(xc.DOT, "12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"),
					},
				},
			},
			// fee
			{
				To: []*xclient.BalanceChange{},
				From: []*xclient.BalanceChange{
					{
						Asset:    "chains/DOT/assets/DOT",
						Contract: "DOT",
						Balance:  xc.NewAmountBlockchainFromUint64(157316518),
						Address:  xclient.NewAddressName(xc.DOT, "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
					},
				},
			},
		},
		Confirmations: 0,
	}
	// don't compare fees as they are calculated from transfers
	res.Fees = nil

	require.Equal(reserialize(correct), reserialize(&res))
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	// TODO: write test
}

func (s *CrosschainTestSuite) TestSubmitTx() {
	// TODO: write test
}

// TODO: write a list of RPC responses that correspond to a whole transaction sequence (metadata, storage,... hash)
// Use subsequences for testing

func mustMarshalJson(data any) string {
	bz, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(bz)
}

func (s *CrosschainTestSuite) TestEstimateTip() {
	require := s.Require()

	type testcase struct {
		responses   []string
		expectedTip uint64
	}

	// precalculate the extrinics and serialize them as substrate mixes binary + json encodings
	extBase := types.NewExtrinsic(types.Call{})
	extBase.Version |= byte(types.ExtrinsicBitSigned)
	sig := make([]byte, 64)
	extBase.Signature = types.ExtrinsicSignatureV4{
		Signature: types.MultiSignature{
			IsEd25519: true,
			AsEd25519: types.NewSignature(sig),
		},
		Nonce: types.NewUCompactFromUInt(100),
		Era:   types.ExtrinsicEra{IsImmortalEra: true},
		Signer: types.MultiAddress{
			IsID: true,
			AsID: types.AccountID(make([]byte, 32)),
		},
		Tip: types.NewUCompactFromUInt(0),
	}
	ext50 := extBase
	ext50.Signature.Tip = types.NewUCompactFromUInt(50)
	ext100 := extBase
	ext100.Signature.Tip = types.NewUCompactFromUInt(100)

	var testcases = []testcase{
		{
			responses: []string{
				test_rpc_meta,
				`{"block":{"extrinsics": [` +
					mustMarshalJson(ext50) + "," +
					mustMarshalJson(ext50) + "," +
					mustMarshalJson(ext50) + "," +
					mustMarshalJson(ext50) + "," +
					mustMarshalJson(ext50) + "," +
					mustMarshalJson(ext100) + "," +
					mustMarshalJson(ext100) +
					`]}}`,
			},
			expectedTip: (50*5 + 100*2) / 7,
		},
	}

	for i, tc := range testcases {
		fmt.Println("== Estimate tip case", i)
		rpc, rpcClose := testtypes.MockJSONRPC(s.T(), tc.responses)
		defer rpcClose()

		client, err := substrate.NewClient(&xc.ChainConfig{
			Chain:      "DOT",
			Driver:     "substrate",
			IndexerUrl: "subscan",
			AuthSecret: "aaa",
			URL:        rpc.URL,
			Decimals:   10,
			ChainID:    0,
		})
		require.NoError(err)
		amt, err := client.EstimateTip(s.Ctx)
		require.NoError(err)
		require.EqualValues(amt, tc.expectedTip)
	}
}
