package client_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic/extensions"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate/client"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/testutil"
	testtypes "github.com/cordialsys/crosschain/testutil"
	txinfo "github.com/cordialsys/crosschain/client/tx-info"
	"github.com/stretchr/testify/require"
)

// reserialize will drop internal fields set by constructors
func reserialize(tx *xclient.TxInfo) *xclient.TxInfo {
	bz, _ := json.Marshal(tx)
	var info xclient.TxInfo
	json.Unmarshal(bz, &info)
	return &info
}

func ref[T any](tx T) *T {
	return &tx
}

// *** RPC & HTTP Test Responses ***
// A copy of the metadata JSON blob from RPC call. This is SCALE-encoded hex that decodes to Substrate Metadata
//
//go:embed test_rpc_meta.json
var RPC_META_RESPONSE string

func TestNewClient(t *testing.T) {
	require := require.New(t)
	_, err := client.NewClient(xc.NewChainConfig(""))
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

func TestBalance(t *testing.T) {
	require := require.New(t)

	type testcase struct {
		responses   []string
		expectedBal uint64
	}
	var testcases = []testcase{
		{
			responses: []string{
				RPC_META_RESPONSE, RPC_META_RESPONSE, asScaleRpcResult(accountInfo(1, 132919986950891)),
			},
			expectedBal: 132919986950891,
		},
		{
			// bittensor's account storage response
			responses: []string{
				RPC_META_RESPONSE,
				RPC_META_RESPONSE,
				`{"jsonrpc":"2.0", "result":"0x06000000000000000100000000000000424a5127000000000000000000000000000000000000000000000000000000000000000000000080","id":1}`,
			},
			expectedBal: 659638850,
		},
	}

	for i, tc := range testcases {
		rpc, rpcClose := testtypes.MockJSONRPC(t, tc.responses)
		defer rpcClose()
		os.Setenv("SUBSTRATE_AAA", "AAA")
		defer os.Unsetenv("SUBSTRATE_AAA")

		chain := xc.NewChainConfig(xc.DOT, "substrate").
			WithUrl(rpc.URL).
			WithIndexer(client.IndexerSubScan, "subscan").
			WithAuth("env:SUBSTRATE_AAA").
			WithChainPrefix("0").
			WithDecimals(10)

		client, err := client.NewClient(chain)
		require.NoError(err)
		require.NotNil(client.DotClient)

		args := xclient.NewBalanceArgs("1598AR2pgoJCWHn3UA2FTemJ74hBWgp7GLyNB4oSkt6vqMno")
		res, err := client.FetchBalance(context.Background(), args)
		fmt.Println("testcase ", i)
		require.NoError(err)
		require.EqualValues(tc.expectedBal, res.Uint64())
	}
}

func TestFetchTxInfo(t *testing.T) {

	type testcase struct {
		hash         string
		responses    []string
		rcpResponses []string
		expectedTx   xclient.TxInfo
		indexerType  string
	}
	var testcases = []testcase{
		{
			hash:        "anything",
			indexerType: client.IndexerSubScan,
			responses: []string{
				`{"code":0,"message":"Success","generated_at":1687790045,"data":{"block_timestamp":1687547412,"block_num":16097417,"extrinsic_index":"16097417-2","call_module_function":"transfer_keep_alive","call_module":"balances","account_id":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","signature":"0x14683b601d4798f437c7278db90763d36a9750ecf84a711206a0f4e30014e9236d22f89f5c90f66cc9c74ff192afe3bb54fdc531a09ee956101c037ff1fd4c01","nonce":0,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"params":[{"name":"dest","type":"sp_runtime:multiaddress:MultiAddress","type_name":"AccountIdLookupOf","value":{"Id":"0x4f3396dd2c6b55498f67ce8883524360347427e30cbc50fb981922de73c4551e"}},{"name":"value","type":"compact\u003cU128\u003e","type_name":"Balance","value":"872321233400"}],"transfer":{"from":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","to":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","module":"balances","amount":"87.23212334","hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"asset_symbol":"DOT","to_account_display":{"address":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","merkle":{"address_type":"hot_wallet","tag_type":"Exchange","tag_subtype":"Optional KYC and AML","tag_name":"ByBit.com"}}},"event":[{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Withdraw","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"157316518\",\"name\":\"amount\"}]","phase":0,"event_idx":40,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Transfer","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"from\"},{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x4f3396dd2c6b55498f67ce8883524360347427e30cbc50fb981922de73c4551e\",\"name\":\"to\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"872321233400\",\"name\":\"amount\"}]","phase":0,"event_idx":41,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Deposit","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x6d6f646c70792f74727372790000000000000000000000000000000000000000\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"125853214\",\"name\":\"amount\"}]","phase":0,"event_idx":42,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"treasury","event_id":"Deposit","params":"[{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"125853214\",\"name\":\"value\"}]","phase":0,"event_idx":43,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"balances","event_id":"Deposit","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x0507535074b7da9ae4989d5010cd0fcb3a33f8a516b721dd1a82c0a44901192e\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"Balance\",\"value\":\"31463304\",\"name\":\"amount\"}]","phase":0,"event_idx":44,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"transactionpayment","event_id":"TransactionFeePaid","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"157316518\",\"name\":\"actual_fee\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"0\",\"name\":\"tip\"}]","phase":0,"event_idx":45,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"system","event_id":"ExtrinsicSuccess","params":"[{\"type\":\"frame_support:dispatch:DispatchInfo\",\"type_name\":\"DispatchInfo\",\"value\":{\"class\":\"Normal\",\"pays_fee\":\"Yes\",\"weight\":{\"proof_size\":3593,\"ref_time\":254268000}},\"name\":\"dispatch_info\"}]","phase":0,"event_idx":46,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0}],"event_count":7,"fee":"157316518","fee_used":"157316518","error":null,"finalized":true,"lifetime":{"birth":16097411,"death":16098435},"tip":"0","account_display":{"address":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"},"block_hash":"0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4","pending":false}}`,
			},
			rcpResponses: []string{
				RPC_META_RESPONSE,
				// seem some endpoints are inconsistent and do not mix scale encoding
				asRpcResult(&types.Header{Number: 16097420}),
			},
			expectedTx: xclient.TxInfo{
				Name:   "chains/DOT/transactions/0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
				Hash:   "0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
				XChain: xc.DOT,
				State:  xclient.Succeeded,
				Final:  true,
				Block: &xclient.Block{
					Chain:  xc.DOT,
					Height: xc.NewAmountBlockchainFromUint64(16097417),
					Hash:   "0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4",
					Time:   testtypes.FromTimeStamp("2023-06-23T19:10:12Z"),
				},
				Movements: []*xclient.Movement{
					{
						XAsset:    "chains/DOT/assets/DOT",
						XContract: "DOT",
						AssetId:   "DOT",
						From: []*xclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(872321233400),
								XAddress:  xclient.NewAddressName(xc.DOT, "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
								AddressId: "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2",
							},
						},
						To: []*xclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(872321233400),
								XAddress:  xclient.NewAddressName(xc.DOT, "12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"),
								AddressId: "12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz",
							},
						},
					},
					// fee
					{
						XAsset:    "chains/DOT/assets/DOT",
						XContract: "DOT",
						AssetId:   "DOT",
						To:        []*xclient.BalanceChange{},
						From: []*xclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(157316518),
								XAddress:  xclient.NewAddressName(xc.DOT, "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
								AddressId: "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2",
							},
						},
						Event: xclient.NewEventFromIndex(0, xclient.MovementVariantFee),
					},
				},
				Confirmations: 3,
			},
		},
		{
			hash:        "anything",
			indexerType: client.IndexerSubQuery,
			responses: []string{
				`{"data":{"extrinsics":{"nodes":[{"id":"3401817-0046","txHash":"0x88a147c2e869ec68827c1db6bba7e5923f555adbc658ad58ad74b730e5eae3e2","tip":"0"}]}}}`,
				`{"data":{"events":{"nodes":[{"module":"system","event":"ExtrinsicSuccess","data":"[{\"weight\":{\"refTime\":286314000,\"proofSize\":3593},\"class\":\"Normal\",\"paysFee\":\"Yes\"}]"},{"module":"transactionPayment","event":"TransactionFeePaid","data":"[\"5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4\",124557,0]"},{"module":"balances","event":"Transfer","data":"[\"5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4\",\"5FpzwkKW7zwbDP5aUuBfzzTCMseCoxwTjxAjK8oKjziQsoyQ\",10000000]"},{"module":"balances","event":"Endowed","data":"[\"5FpzwkKW7zwbDP5aUuBfzzTCMseCoxwTjxAjK8oKjziQsoyQ\",10000000]"},{"module":"system","event":"NewAccount","data":"[\"5FpzwkKW7zwbDP5aUuBfzzTCMseCoxwTjxAjK8oKjziQsoyQ\"]"},{"module":"balances","event":"Withdraw","data":"[\"5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4\",124557]"}]},"blocks":{"nodes":[{"timestamp":"2024-07-17T01:17:24.002","hash":"0x9d264b95980880a3ce28024e093af7f39c434bfc2dd0472fffdcbb924a369b25"}]}}}`,
			},
			rcpResponses: []string{
				RPC_META_RESPONSE,
				// seem some endpoints are inconsistent and do not mix scale encoding
				asRpcResult(&types.Header{Number: 3401825}),
			},
			expectedTx: xclient.TxInfo{
				Name:   "chains/TAO/transactions/0x88a147c2e869ec68827c1db6bba7e5923f555adbc658ad58ad74b730e5eae3e2",
				Hash:   "0x88a147c2e869ec68827c1db6bba7e5923f555adbc658ad58ad74b730e5eae3e2",
				XChain: xc.TAO,
				State:  xclient.Succeeded,
				Final:  true,
				Block: &xclient.Block{
					Chain:  xc.TAO,
					Height: xc.NewAmountBlockchainFromUint64(3401817),
					Hash:   "0x9d264b95980880a3ce28024e093af7f39c434bfc2dd0472fffdcbb924a369b25",
					Time:   testtypes.FromTimeStamp("2024-07-17T01:17:24Z"),
				},
				Movements: []*xclient.Movement{
					{
						XAsset:    "chains/TAO/assets/TAO",
						XContract: "TAO",
						AssetId:   "TAO",
						From: []*xclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(10000000),
								XAddress:  xclient.NewAddressName(xc.TAO, "5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4"),
								AddressId: "5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4",
							},
						},
						To: []*xclient.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(10000000),
								XAddress:  xclient.NewAddressName(xc.TAO, "5FpzwkKW7zwbDP5aUuBfzzTCMseCoxwTjxAjK8oKjziQsoyQ"),
								AddressId: "5FpzwkKW7zwbDP5aUuBfzzTCMseCoxwTjxAjK8oKjziQsoyQ",
							},
						},
					},
					{
						XAsset:    "chains/TAO/assets/TAO",
						XContract: "TAO",
						AssetId:   "TAO",
						From: []*xclient.BalanceChange{
							// fee
							{
								Balance:   xc.NewAmountBlockchainFromUint64(124557),
								XAddress:  xclient.NewAddressName(xc.TAO, "5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4"),
								AddressId: "5HP3f2acWoEKj9AVZGa9DtA4bykmoSBSovoSZTL2vD2DgqV4",
							},
						},
						To:    []*xclient.BalanceChange{},
						Event: xclient.NewEventFromIndex(0, xclient.MovementVariantFee),
					},
				},
				Confirmations: 8,
			},
		},

		{
			// extrinsic failed
			hash:        "anything",
			indexerType: client.IndexerSubScan,
			responses: []string{
				`{"code":0,"message":"Success","generated_at":1687790045,"data":{"block_timestamp":1687547412,"block_num":16097417,"extrinsic_index":"16097417-2","call_module_function":"transfer_keep_alive","call_module":"balances","account_id":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","signature":"0x14683b601d4798f437c7278db90763d36a9750ecf84a711206a0f4e30014e9236d22f89f5c90f66cc9c74ff192afe3bb54fdc531a09ee956101c037ff1fd4c01","nonce":0,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"params":
					[{"name":"dest","type":"sp_runtime:multiaddress:MultiAddress","type_name":"AccountIdLookupOf","value":{"Id":"0x4f3396dd2c6b55498f67ce8883524360347427e30cbc50fb981922de73c4551e"}},{"name":"value","type":"compact\u003cU128\u003e","type_name":"Balance","value":"872321233400"}],"transfer":{"from":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2","to":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","module":"balances","amount":"87.23212334","hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","success":true,"asset_symbol":"DOT","to_account_display":{"address":"12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz","merkle":{"address_type":"hot_wallet","tag_type":"Exchange","tag_subtype":"Optional KYC and AML","tag_name":"ByBit.com"}}},
					"event":[
					{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"transactionpayment","event_id":"TransactionFeePaid","params":"[{\"type\":\"[U8; 32]\",\"type_name\":\"AccountId\",\"value\":\"0x5df87265f6ce0c1914eb15c3bdacf6722373e69a1a8d90ac0bc58f5e7fdd246d\",\"name\":\"who\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"157316518\",\"name\":\"actual_fee\"},{\"type\":\"U128\",\"type_name\":\"BalanceOf\",\"value\":\"0\",\"name\":\"tip\"}]","phase":0,"event_idx":45,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0},
					{"event_index":"16097417-2","block_num":16097417,"extrinsic_idx":2,"module_id":"system","event_id":"ExtrinsicFailed","params":"[{\"type\":\"frame_support:dispatch:DispatchInfo\",\"type_name\":\"DispatchInfo\",\"value\":{\"class\":\"Normal\",\"pays_fee\":\"Yes\",\"weight\":{\"proof_size\":3593,\"ref_time\":254268000}},\"name\":\"dispatch_info\"}]","phase":0,"event_idx":46,"extrinsic_hash":"0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb","finalized":true,"block_timestamp":0}
					],"event_count":2,"fee":"157316518","fee_used":"157316518","error":null,"finalized":true,"lifetime":{"birth":16097411,"death":16098435},"tip":"0","account_display":{"address":"138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"},"block_hash":"0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4","pending":false}}`,
			},
			rcpResponses: []string{
				RPC_META_RESPONSE,
				// seem some endpoints are inconsistent and do not mix scale encoding
				asRpcResult(&types.Header{Number: 16097420}),
			},
			expectedTx: xclient.TxInfo{
				Name:   "chains/DOT/transactions/0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
				Hash:   "0x47cf6465b5288b5bb1e1107ff9f8a7ac9e690dc6eead5fb3fa12f47213c028cb",
				XChain: xc.DOT,
				State:  xclient.Failed,
				Final:  true,
				Block: &xclient.Block{
					Chain:  xc.DOT,
					Height: xc.NewAmountBlockchainFromUint64(16097417),
					Hash:   "0x5031ce3733226cfd2c877811d0779760cf3cc29f0ba0cea500ef380c19e72fa4",
					Time:   testtypes.FromTimeStamp("2023-06-23T19:10:12Z"),
				},
				Movements: []*xclient.Movement{
					// fee
					{
						XAsset:    "chains/DOT/assets/DOT",
						XContract: "DOT",
						AssetId:   "DOT",
						To:        []*xclient.BalanceChange{},
						From: []*xclient.BalanceChange{
							{

								Balance:   xc.NewAmountBlockchainFromUint64(157316518),
								XAddress:  xclient.NewAddressName(xc.DOT, "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2"),
								AddressId: "138DFvwTQfQN9ZttPm1HDBVRcEwGfsPxdWRfKktrquziu8c2",
							},
						},
						Event: xclient.NewEventFromIndex(0, xclient.MovementVariantFee),
					},
				},
				Confirmations: 3,
				Error:         ref("transaction failed"),
			},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			require := require.New(t)
			// for the indexer to download the transaction
			http, httpClose := testtypes.MockHTTP(t, tc.responses, 200)
			defer httpClose()
			// for the client fetching the lastest block height
			rpc, rpcClose := testtypes.MockJSONRPC(t, tc.rcpResponses)
			defer rpcClose()

			os.Setenv("SUBSTRATE_AAA", "AAA")
			defer os.Unsetenv("SUBSTRATE_AAA")

			chain := xc.NewChainConfig(tc.expectedTx.XChain, "substrate").
				WithUrl(rpc.URL).
				WithIndexer(tc.indexerType, http.URL).
				WithAuth("env:SUBSTRATE_AAA").
				WithChainPrefix("0").
				WithDecimals(10)

			client, err := client.NewClient(chain)
			require.NoError(err)
			require.NotNil(client)

			args := txinfo.NewArgs(xc.TxHash(tc.hash))
			res, err := client.FetchTxInfo(context.Background(), args)
			require.NoError(err)

			// don't compare fees as they are calculated from transfers
			res.Fees = nil

			testutil.TxInfoEqual(t, tc.expectedTx, res)
		})
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

func TestFetchTxInput(t *testing.T) {
	require := require.New(t)

	exts := exampleExtrinsics([]int{50, 100})
	ext50 := exts[0]

	expectedMeta := tx_input.Metadata{
		Calls: []*tx_input.CallMeta{
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
		expectedInput tx_input.TxInput
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
			expectedInput: tx_input.TxInput{
				TxInputEnvelope: tx_input.NewTxInput().TxInputEnvelope,
				Meta:            expectedMeta,
				Rv: types.RuntimeVersion{
					APIs: []types.RuntimeVersionAPI{},
				},
				CurrentHeight: 16097420,
				Nonce:         22,
			},
		},
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
				// bittensor account info
				`{"jsonrpc":"2.0", "result":"0x06000000000000000100000000000000424a5127000000000000000000000000000000000000000000000000000000000000000000000080","id":1}`,
				// get latest block info
				`{"block":{"extrinsics": [` + mustMarshalScaleJson(ext50) + `]}}`,
			},
			expectedInput: tx_input.TxInput{
				TxInputEnvelope: tx_input.NewTxInput().TxInputEnvelope,
				Meta:            expectedMeta,
				Rv: types.RuntimeVersion{
					APIs: []types.RuntimeVersionAPI{},
				},
				CurrentHeight: 16097420,
				Nonce:         6,
			},
		},
	}

	for i, tc := range testcases {
		fmt.Println("== tx-input test case", i)
		// for the indexer to download the transaction
		// for the client fetching the lastest block height
		rpc, rpcClose := testtypes.MockJSONRPC(t, tc.responses)
		defer rpcClose()
		os.Setenv("SUBSTRATE_AAA", "AAA")
		defer os.Unsetenv("SUBSTRATE_AAA")
		chain := xc.NewChainConfig("DOT", "substrate").
			WithUrl(rpc.URL).
			WithIndexer("", "aaa").
			WithAuth("env:SUBSTRATE_AAA").
			WithChainPrefix("0").
			WithDecimals(10)

		client, err := client.NewClient(chain)
		require.NoError(err)
		require.NotNil(client)

		addr := "12nr7GiDrYHzAYT9L8HdeXnMfWcBuYfAXpgfzf3upujeCciz"
		res, err := client.FetchLegacyTxInput(context.Background(), xc.Address(addr), xc.Address(addr))
		require.NoError(err)

		require.Equal(&tc.expectedInput, res)
	}
}

func TestEstimateTip(t *testing.T) {
	require := require.New(t)

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
		rpc, rpcClose := testtypes.MockJSONRPC(t, tc.responses)
		defer rpcClose()

		os.Setenv("SUBSTRATE_AAA", "AAA")
		defer os.Unsetenv("SUBSTRATE_AAA")
		chain := xc.NewChainConfig("DOT", "substrate").
			WithUrl(rpc.URL).
			WithIndexer("", "subscan").
			WithAuth("env:SUBSTRATE_AAA").
			WithDecimals(10)

		client, err := client.NewClient(chain)
		require.NoError(err)
		amt, err := client.EstimateTip(context.Background())
		require.NoError(err)
		require.EqualValues(amt, tc.expectedTip)
	}
}
