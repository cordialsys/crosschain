package blockbook_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"

	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	"github.com/stretchr/testify/suite"
)

type ClientTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *ClientTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestBlockbookTestSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

func (s *ClientTestSuite) TestFetchTxInput() {
	require := s.Require()
	// 2392235
	type testcase struct {
		utxos         []int
		targetAmount  int
		expectedTotal int
		expectedLen   int
	}
	testcases := []testcase{
		{
			utxos:         []int{2_392_235},
			targetAmount:  5_000_000,
			expectedTotal: 2_392_235,
			expectedLen:   1,
		},
		{
			utxos:         []int{1_000_000, 3_000_000},
			targetAmount:  5_000_000,
			expectedTotal: 4_000_000,
			expectedLen:   2,
		},
		{
			utxos:         []int{2_000_000, 1_000_000, 3_000_000},
			targetAmount:  5_000_000,
			expectedTotal: 6_000_000,
			expectedLen:   3,
		},
		{
			// should include dust utxo's, up to 10
			utxos:         []int{2_000_000, 1_000_000, 3_000_000, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			targetAmount:  5_000_000,
			expectedTotal: 5_000_000 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8,
			expectedLen:   10,
		},
		{
			// order input shouldn't matter
			utxos:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 2_000_000, 1_000_000, 3_000_000},
			targetAmount:  5_000_000,
			expectedTotal: 5_000_000 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8,
			expectedLen:   10,
		},
	}
	for _, v := range testcases {
		utxoJsons := []string{}
		for i, utxo := range v.utxos {
			s := fmt.Sprintf(`{"height":100,"confirmations":100,"txid":"c4979460bb03a1877bbf23571c83edbd02cb4da20049916fa6c5fbf77470e027","vout":%d,"value":"%d"}`, i+1, utxo)
			utxoJsons = append(utxoJsons, s)
		}

		server, close := testtypes.MockHTTP(s.T(), []string{
			// /api/v2/utxo
			fmt.Sprintf(
				"[" + strings.Join(utxoJsons, ",") + "]",
			),
			// /api/v2/estimatefee
			`{"result": "0.00007998"}`,
		}, 200)
		defer close()
		asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("testnet").WithProvider(string(bitcoin.Blockbook)).WithMinGasPrice(15)
		client, _ := bitcoin.NewClient(asset)

		from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
		to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
		args := buildertest.MustNewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(uint64(v.targetAmount)))
		input, err := client.FetchTransferInput(s.Ctx, args)
		require.NotNil(input)
		// optimize the utxo amounts
		input.(xc.TxInputWithAmount).SetAmount(xc.NewAmountBlockchainFromUint64(uint64(v.targetAmount)))

		fmt.Println(input)
		fmt.Println(err)
		require.NoError(err)
		btcInput := input.(*tx_input.TxInput)
		fmt.Println(btcInput)
		if len(btcInput.UnspentOutputs) != v.expectedLen {
			require.Fail("not expected", "%d vs %d", len(btcInput.UnspentOutputs), v.expectedLen)
		}
		require.Len(btcInput.UnspentOutputs, v.expectedLen)
		total := btcInput.SumUtxo()
		require.EqualValues(v.expectedTotal, total.Uint64())
		require.NotZero(btcInput.UnspentOutputs[0].Index)
		// string should be reversed
		require.EqualValues("27e07074f7fbc5a66f914900a24dcb02bded831c5723bf7b87a103bb609497c4", hex.EncodeToString(btcInput.UnspentOutputs[0].Hash))
		require.LessOrEqual(uint64(15), btcInput.GasPricePerByte.Uint64())
		require.GreaterOrEqual(uint64(30), btcInput.GasPricePerByte.Uint64())

		// should be sorted with the largest utxo used first
		firstValue := btcInput.UnspentOutputs[0].Value.Uint64()
		sort.Slice(btcInput.UnspentOutputs, func(i, j int) bool {
			return btcInput.UnspentOutputs[i].Value.Uint64() > btcInput.UnspentOutputs[j].Value.Uint64()
		})
		require.Equal(firstValue, btcInput.UnspentOutputs[0].Value.Uint64())

	}
}

func (s *ClientTestSuite) TestFetchTxInfo() {
	require := s.Require()
	server, close := testtypes.MockHTTP(s.T(), []string{
		// tx
		`{"txid":"999be3740a25dc6def2e62df25be1387011c22bbf3a4b1b448ff1180e86e64f2","version":2,"vin":[{"txid":"6096941b53496f1c2196a8e5b589c01a0dd1f0b9b6754da5861d485b339b9436","vout":1,"sequence":4294967293,"n":0,"addresses":["bc1p6q4qhp9j008m2wvxjp0ffzc7ulkvzn8awaqgxjpcpzyxnlpfhrusst6t8h"],"isAddress":true,"value":"12651"}],"vout":[{"value":"546","n":0,"hex":"001436775d21d459d18cbf3d28b4eaaab0280cbcae19","addresses":["bc1qxem46gw5t8gce0ea9z6w424s9qxtetse5d69uu"],"isAddress":true},{"value":"0","n":1,"hex":"6a5d061486f533144d","addresses":[],"isAddress":false},{"value":"8663","n":2,"hex":"5120d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f9","addresses":["bc1p6q4qhp9j008m2wvxjp0ffzc7ulkvzn8awaqgxjpcpzyxnlpfhrusst6t8h"],"isAddress":true}],"blockHeight":850509,"confirmations":0,"blockTime":1720038342,"value":"9209","valueIn":"12651","fees":"3442","hex":"0200000000010136949b335b481d86a54d75b6b9f0d10d1ac089b5e5a896211c6f49531b9496600100000000fdffffff03220200000000000016001436775d21d459d18cbf3d28b4eaaab0280cbcae190000000000000000096a5d061486f533144dd721000000000000225120d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f901409fbf530c09ae37186996f2929b80a7028d3cc3176598e032af890eb2d053a518cefbe041fa7864c751b68a5f87f9b8f78ee3fd0aae353799d306078ec59301fe00000000","rbf":true,"coinSpecificData":{"txid":"999be3740a25dc6def2e62df25be1387011c22bbf3a4b1b448ff1180e86e64f2","hash":"7d414e1099cb205612e1b720d9b0665ab4a08746105f5a2ba581da3e8779f19e","version":2,"size":211,"vsize":160,"weight":640,"locktime":0,"vin":[{"txid":"6096941b53496f1c2196a8e5b589c01a0dd1f0b9b6754da5861d485b339b9436","vout":1,"scriptSig":{"asm":"","hex":""},"txinwitness":["9fbf530c09ae37186996f2929b80a7028d3cc3176598e032af890eb2d053a518cefbe041fa7864c751b68a5f87f9b8f78ee3fd0aae353799d306078ec59301fe"],"sequence":4294967293}],"vout":[{"value":0.00000546,"n":0,"scriptPubKey":{"asm":"0 36775d21d459d18cbf3d28b4eaaab0280cbcae19","desc":"addr(bc1qxem46gw5t8gce0ea9z6w424s9qxtetse5d69uu)#ncs86s49","hex":"001436775d21d459d18cbf3d28b4eaaab0280cbcae19","address":"bc1qxem46gw5t8gce0ea9z6w424s9qxtetse5d69uu","type":"witness_v0_keyhash"}},{"value":0.00000000,"n":1,"scriptPubKey":{"asm":"OP_RETURN 13 1486f533144d","desc":"raw(6a5d061486f533144d)#3s3j5jcn","hex":"6a5d061486f533144d","type":"nulldata"}},{"value":0.00008663,"n":2,"scriptPubKey":{"asm":"1 d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f9","desc":"rawtr(d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f9)#cemc67w3","hex":"5120d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f9","address":"bc1p6q4qhp9j008m2wvxjp0ffzc7ulkvzn8awaqgxjpcpzyxnlpfhrusst6t8h","type":"witness_v1_taproot"}}],"hex":"0200000000010136949b335b481d86a54d75b6b9f0d10d1ac089b5e5a896211c6f49531b9496600100000000fdffffff03220200000000000016001436775d21d459d18cbf3d28b4eaaab0280cbcae190000000000000000096a5d061486f533144dd721000000000000225120d02a0b84b27bcfb53986905e948b1ee7ecc14cfd7740834838088869fc29b8f901409fbf530c09ae37186996f2929b80a7028d3cc3176598e032af890eb2d053a518cefbe041fa7864c751b68a5f87f9b8f78ee3fd0aae353799d306078ec59301fe00000000"}}`,
		// stats
		`{"blockbook":{"coin":"Bitcoin","host":"2387762225de","version":"unknown","gitCommit":"unknown","buildTime":"unknown","syncMode":true,"initialSync":false,"inSync":true,"bestHeight":850578,"lastBlockTime":"2024-07-03T20:18:50.835054532Z","inSyncMempool":true,"lastMempoolTime":"2024-07-03T20:25:42.687082833Z","mempoolSize":55449,"decimals":8,"dbSize":499462256929,"about":"Blockbook - blockchain indexer for Trezor wallet https://trezor.io/. Do not use for any other purpose."},"backend":{"chain":"main","blocks":850578,"headers":850578,"bestBlockHash":"00000000000000000001e3bc7fc4fdf42af1968aa9f1c9d95a3089b0943efa12","difficulty":"83675262295059.91","sizeOnDisk":662338961933,"version":"270100","subversion":"/Satoshi:27.1.0/","protocolVersion":"70016"}}`,
	}, 200)
	defer close()
	asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("testnet").WithProvider(string(bitcoin.Blockbook))
	client, err := bitcoin.NewClient(asset)
	require.NoError(err)
	info, err := client.FetchLegacyTxInfo(s.Ctx, xc.TxHash("227178d784150211e8ea5a586ee75bc97655e61f02bc8c07557e475cfecea3cd"))
	require.NotNil(info)
	require.NoError(err)
	require.EqualValues(546, info.Amount.Uint64())
	require.EqualValues("227178d784150211e8ea5a586ee75bc97655e61f02bc8c07557e475cfecea3cd", info.TxID)
	require.Len(info.Sources, 1)
	// destination should not include the change
	require.Len(info.Destinations, 1)
	require.EqualValues("bc1qxem46gw5t8gce0ea9z6w424s9qxtetse5d69uu", info.Destinations[0].Address)
	require.EqualValues("bc1p6q4qhp9j008m2wvxjp0ffzc7ulkvzn8awaqgxjpcpzyxnlpfhrusst6t8h", info.Sources[0].Address)
	require.EqualValues(546, info.Destinations[0].Amount.Uint64())
	require.EqualValues(xc.TxStatusSuccess, info.Status)
	require.EqualValues(70, info.Confirmations)
	require.EqualValues(3442, info.Fee.Uint64())
}

func (s *ClientTestSuite) TestNotFoundFetchTxInfo() {
	require := s.Require()
	server, close := testtypes.MockHTTP(s.T(), []string{
		// tx
		`{"error":"Transaction '5065d8469f4d02d58c002d234127ab6966fb36737b3fc22c08f0866c01fac38b' not found"}`,
	}, 400)
	defer close()
	asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("testnet").WithProvider(string(bitcoin.Blockbook))
	client, err := bitcoin.NewClient(asset)
	require.NoError(err)
	_, err = client.FetchLegacyTxInfo(s.Ctx, xc.TxHash("227178d784150211e8ea5a586ee75bc97655e61f02bc8c07557e475cfecea3cd"))
	require.Error(err)

	require.ErrorContains(err, "TransactionNotFound:")
}
