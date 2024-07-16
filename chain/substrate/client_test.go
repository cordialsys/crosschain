package substrate_test

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic/extensions"
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
var RPC_META_RESPONSE string

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	_, err := substrate.NewClient(&xc.ChainConfig{})
	require.Error(err)
}

// Marshal scale encoding as valid json hex string
func mustMarshalScaleJson(data any) string {
	s, err := codec.EncodeToHex(data)
	if err != nil {
		panic(err)
	}
	return mustMarshalJson(s)
}

// Wrap into rpc json result
func asScaleRpcResult(data any) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0", "result":%s,"id":1}`, mustMarshalScaleJson(data))
}

func mustMarshalJson(data any) string {
	bz, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(bz)
}
func asRpcResult(data any) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0", "result":%s,"id":1}`, mustMarshalJson(data))
}

func accountInfo(nonce, balance uint64) *types.AccountInfo {
	ai := &types.AccountInfo{
		Nonce: types.NewU32(uint32(nonce)),
		Data: struct {
			Free       types.U128
			Reserved   types.U128
			MiscFrozen types.U128
			Flags      types.U128
		}{
			Free: types.NewU128(*big.NewInt(int64(balance))),
		},
	}
	return ai
}

func (s *CrosschainTestSuite) TestBalance() {
	require := s.Require()

	// Note that these RPC calls do have IDs, which the client library increments per call but doesn't care if they
	// don't match up. The test json files all use ID 1.

	rpc, rpcClose := testtypes.MockJSONRPC(s.T(), []string{RPC_META_RESPONSE, RPC_META_RESPONSE, asScaleRpcResult(accountInfo(1, 132919986950891))})
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
	require.NoError(err)
	require.NotNil(client.DotClient)

	res, err := client.FetchBalance(s.Ctx, "1598AR2pgoJCWHn3UA2FTemJ74hBWgp7GLyNB4oSkt6vqMno")
	require.NoError(err)
	require.Equal(uint64(132919986950891), res.Uint64())
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()

	type testcase struct {
		hash         string
		responses    []string
		rcpResponses []string
		expectedTx   xclient.TxInfo
	}
	var testcases = []testcase{
		{
			hash: "47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
			responses: []string{
				`{"code":0,"message":"Success","generated_at":1687790045,"data":{"block_timestamp":1687547412,"block_num":16097417,"extrinsic_index":"16097417-2","call_module_function":"transfer_keep_alive","call_module":"balances","account_id":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","signature":"0x14683b601d4798f437c7278db90763d36a9750ecf84a711206a0f4e30014e9236d22f89f5c90f66cc9c74ff192afe3bb54fdc531a09ee956101c037ff1fd4c01","nonce":0,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"params":[{"name":"dest","type":"sp_runtime:multiaddress:MultiAddress","type_name":"AccountIdLookupOf","value":{"Id":"0x4f3396dd2c6b55498f67ce8883524360347427e30cbc50fb981922de73c4551e"}},{"name":"value","type":"compact\u003cU128\u003e","type_name":"Balance","value":"872321233400"}],"transfer":{"from":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","to":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","module":"balances","amount":"87.23212334","hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"asset_symbol":"DOT","to_account_display":{"address":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","merkle":{"address_type":"hot_wallet","tag_type":"Exchange","tag_subtype":"Optional KYC and AML","tag_name":"ByBit.com"}}},"event":[{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Withdraw","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"157316518\",\"name\":\"amount\"}]","phase":0,"event_idx":40,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Transfer","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"from\"},{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x4f3396dd2c6b55498f67ce8883524360347427e30cbc50fb981922de73c4551e\",\"name\":\"to\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"872321233400\",\"name\":\"amount\"}]","phase":0,"event_idx":41,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Deposit","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x6d6f646c70792f74727372790000000000000000000000000000000000000000\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"125853214\",\"name\":\"amount\"}]","phase":0,"event_idx":42,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"treasury","event_id":"Deposit","params":"[{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"125853214\",\"name\":\"value\"}]","phase":0,"event_idx":43,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Deposit","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x0507535074b7da9ae4989d5010cd0fcb3a33f8a516b721dd1a82c0a44901192e\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"31463304\",\"name\":\"amount\"}]","phase":0,"event_idx":44,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"transactionpayment","event_id":"TransactionFeePaid","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"157316518\",\"name\":\"actual_fee\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"0\",\"name\":\"tip\"}]","phase":0,"event_idx":45,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"system","event_id":"ExtrinsicSuccess","params":"[{\"type\":\"frame_support:dispatch:DispatchInfo\",\"type_name\":\"DispatchInfo\",\"value\":{\"class\":\"Normal\",\"pays_fee\":\"Yes\",\"weight\":{\"proof_size\":3593,\"ref_time\":254268000}},\"name\":\"dispatch_info\"}]","phase":0,"event_idx":46,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0}],"event_count":7,"fee":"157316518","fee_used":"157316518","error":null,"finalized":true,"lifetime":{"birth":16097411,"death":16098435},"tip":"0","account_display":{"address":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"},"block_hash":"0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4","pending":false}}`,
			},
			rcpResponses: []string{
				RPC_META_RESPONSE,
				// seem some endpoints are inconsistent and do not mix scale encoding
				asRpcResult(&types.Header{Number: 16097420}),
			},
			expectedTx: xclient.TxInfo{
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
				Confirmations: 3,
			},
		},
	}

	for i, tc := range testcases {
		fmt.Println("== tx-info test case", i)
		// for the indexer to download the transaction
		http, httpClose := testtypes.MockHTTP(s.T(), tc.responses, 200)
		defer httpClose()
		// for the client fetching the lastest block height
		rpc, rpcClose := testtypes.MockJSONRPC(s.T(), tc.rcpResponses)
		defer rpcClose()

		client, err := substrate.NewClient(&xc.ChainConfig{
			Chain:       "DOT",
			Driver:      "substrate",
			URL:         rpc.URL,
			IndexerUrl:  http.URL,
			AuthSecret:  "aaa",
			ChainPrefix: "0",
			Decimals:    10,
			ChainID:     0,
			ExplorerURL: http.URL,
		})
		require.NoError(err)
		require.NotNil(client)

		res, err := client.FetchTxInfo(s.Ctx, xc.TxHash(tc.hash))
		require.NoError(err)

		// don't compare fees as they are calculated from transfers
		res.Fees = nil

		require.Equal(reserialize(&tc.expectedTx), reserialize(&res))
	}
}

func exampleExtrinsics(tips []int) []types.Extrinsic {
	exts := []types.Extrinsic{}
	for _, tip := range tips {
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
			Tip: types.NewUCompactFromUInt(uint64(tip)),
		}
		exts = append(exts, extBase)
	}
	return exts
}

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()

	exts := exampleExtrinsics([]int{50, 100})
	ext50 := exts[0]

	expectedMeta := substrate.Metadata{
		Calls: []*substrate.CallMeta{
			{
				Name:         "Balances.transfer_keep_alive",
				SectionIndex: 5,
				MethodIndex:  3,
			},
		},
		SignedExtensions: []extensions.SignedExtensionName{
			"CheckNonZeroSender",
			"CheckSpecVersion",
			"CheckTxVersion",
			"CheckGenesis",
			"CheckMortality",
			"CheckNonce",
			"CheckWeight",
			"ChargeTransactionPayment",
			"PrevalidateAttests",
		},
	}

	type testcase struct {
		responses     []string
		expectedInput substrate.TxInput
	}
	var testcases = []testcase{
		{
			responses: []string{
				// meta
				RPC_META_RESPONSE,
				RPC_META_RESPONSE,
				// get genesis block hash
				asRpcResult(types.NewHash(make([]byte, 32))),
				// get runtime version
				asRpcResult(types.NewRuntimeVersion()),
				// header
				asRpcResult(&types.Header{Number: 16097420}),
				// get current block hash
				asRpcResult(types.NewHash(make([]byte, 32))),
				// get account info
				asScaleRpcResult(accountInfo(22, 100)),
				// get latest block info
				`{"block":{"extrinsics": [` + mustMarshalScaleJson(ext50) + `]}}`,
			},
			expectedInput: substrate.TxInput{
				TxInputEnvelope: substrate.NewTxInput().TxInputEnvelope,
				Meta:            expectedMeta,
				Rv: types.RuntimeVersion{
					APIs: []types.RuntimeVersionAPI{},
				},
				CurrentHeight: 16097420,
				Nonce:         22,
			},
		},
	}

	for i, tc := range testcases {
		fmt.Println("== tx-input test case", i)
		// for the indexer to download the transaction
		// for the client fetching the lastest block height
		rpc, rpcClose := testtypes.MockJSONRPC(s.T(), tc.responses)
		defer rpcClose()

		client, err := substrate.NewClient(&xc.ChainConfig{
			Chain:       "DOT",
			Driver:      "substrate",
			URL:         rpc.URL,
			IndexerUrl:  "aaa",
			AuthSecret:  "aaa",
			ChainPrefix: "0",
			Decimals:    10,
			ChainID:     0,
		})
		require.NoError(err)
		require.NotNil(client)

		addr := "12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"
		res, err := client.FetchTxInput(s.Ctx, xc.Address(addr), xc.Address(addr))
		require.NoError(err)

		require.Equal(&tc.expectedInput, res)
	}
}

func (s *CrosschainTestSuite) TestEstimateTip() {
	require := s.Require()

	type testcase struct {
		responses   []string
		expectedTip uint64
	}

	exts := exampleExtrinsics([]int{50, 100})
	ext50 := exts[0]
	ext100 := exts[1]

	var testcases = []testcase{
		{
			responses: []string{
				RPC_META_RESPONSE,
				`{"block":{"extrinsics": [` +
					mustMarshalScaleJson(ext50) + "," +
					mustMarshalScaleJson(ext50) + "," +
					mustMarshalScaleJson(ext50) + "," +
					mustMarshalScaleJson(ext50) + "," +
					mustMarshalScaleJson(ext50) + "," +
					mustMarshalScaleJson(ext100) + "," +
					mustMarshalScaleJson(ext100) +
					`]}}`,
			},
			expectedTip: (50*5 + 100*2) / 7,
		},
		{
			// use zero tip when there are few extrinisics
			responses: []string{
				RPC_META_RESPONSE,
				`{"block":{"extrinsics": [` +
					mustMarshalScaleJson(ext50) +
					`]}}`,
			},
			expectedTip: 0,
		},
		{
			responses: []string{
				RPC_META_RESPONSE,
				`{"block":{"extrinsics": [` +
					`]}}`,
			},
			expectedTip: 0,
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
