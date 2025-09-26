package client_test

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
	testtypes "github.com/cordialsys/crosschain/testutil"
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
			expectedTotal: 3_000_000 + 2_000_000 + 1_000_000 + 11 + 10 + 9 + 8 + 7 + 6 + 5,
			expectedLen:   10,
		},
		{
			// order input shouldn't matter
			utxos:         []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 2_000_000, 1_000_000, 3_000_000},
			targetAmount:  5_000_000,
			expectedTotal: 3_000_000 + 2_000_000 + 1_000_000 + 11 + 10 + 9 + 8 + 7 + 6 + 5,
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
			"[" + strings.Join(utxoJsons, ",") + "]",
			// estimatesmartfee
			`{"jsonrpc":"2.0","result":{"feerate":0.00004965,"blocks":4},"id":1}`,
		}, 200)
		defer close()
		asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("testnet").WithProvider(string(bitcoin.Blockbook))
		client, _ := bitcoin.NewClient(asset)

		from := xc.Address("mpjwFvP88ZwAt3wEHY6irKkGhxcsv22BP6")
		to := xc.Address("tb1qtpqqpgadjr2q3f4wrgd6ndclqtfg7cz5evtvs0")
		args := buildertest.MustNewTransferArgs(asset.ChainBaseConfig, from, to, xc.NewAmountBlockchainFromUint64(uint64(v.targetAmount)))
		input, err := client.FetchTransferInput(s.Ctx, args)
		require.NotNil(input)
		// optimize the utxo amounts
		input.(*tx_input.TxInput).SetAmount(xc.NewAmountBlockchainFromUint64(uint64(v.targetAmount)))

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
		require.LessOrEqual(float64(3), btcInput.GasPricePerByte.Decimal().InexactFloat64())
		require.GreaterOrEqual(float64(15), btcInput.GasPricePerByte.Decimal().InexactFloat64())

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
		//  getrawtransaction tx
		`{"jsonrpc":"2.0","result":{"txid":"d802946fc7180d899e3006fd9b6991b0f42e2f4af9aec697ce811da068105d2e","hash":"f4803af7703c328f4fcc0a2d1a5221c658e28358359f5e6dac2e53ca34223826","version":2,"size":222,"vsize":141,"weight":561,"locktime":916477,"vin":[{"txid":"8c223b0ea1fe3413f63ca6ddf7d72229d2ed5aeaa059dce120e1b5a9ea761653","vout":0,"scriptSig":{"asm":"","hex":""},"txinwitness":["3044022035375cfa7701d1aabd12529eee51af1f9ab78bfe7ddc6ef9dbfb9a15e00b4f650220444f41bc3021b6d4b266eb477e381bc7f88872931c6b2a18de121f077d90b2b801","026d24764cc08ab444797aa310b8b648675a21d77095159c23fac36823967d92bb"],"sequence":4294967293}],"vout":[{"value":0.00061440,"n":0,"scriptPubKey":{"asm":"0 89f634e16913ab2c1f52e6ab42d7eb090ffc30e2","desc":"addr(bc1q38mrfctfzw4jc86ju64594ltpy8lcv8zyd6fxu)#taq55a56","hex":"001489f634e16913ab2c1f52e6ab42d7eb090ffc30e2","address":"bc1q38mrfctfzw4jc86ju64594ltpy8lcv8zyd6fxu","type":"witness_v0_keyhash"}},{"value":0.00164270,"n":1,"scriptPubKey":{"asm":"0 d68ee60e667da8ceb09e19748a8b9974d9a14acb","desc":"addr(bc1q668wvrnx0k5vavy7r96g4zuewnv6zjktzmpel0)#0etzs6cw","hex":"0014d68ee60e667da8ceb09e19748a8b9974d9a14acb","address":"bc1q668wvrnx0k5vavy7r96g4zuewnv6zjktzmpel0","type":"witness_v0_keyhash"}}],"hex":"02000000000101531676eaa9b5e120e1dc59a0ea5aedd22922d7f7dda63cf61334fea10e3b228c0000000000fdffffff0200f000000000000016001489f634e16913ab2c1f52e6ab42d7eb090ffc30e2ae81020000000000160014d68ee60e667da8ceb09e19748a8b9974d9a14acb02473044022035375cfa7701d1aabd12529eee51af1f9ab78bfe7ddc6ef9dbfb9a15e00b4f650220444f41bc3021b6d4b266eb477e381bc7f88872931c6b2a18de121f077d90b2b80121026d24764cc08ab444797aa310b8b648675a21d77095159c23fac36823967d92bbfdfb0d00","blockhash":"000000000000000000005223837ff93667f39a926a583a03bfedeee8f9da59af","confirmations":3,"time":1758899790,"blocktime":1758899790},"id":1}`,
		// getrawtransactionk tx of spent utxo
		`{"jsonrpc":"2.0","result":{"txid":"8c223b0ea1fe3413f63ca6ddf7d72229d2ed5aeaa059dce120e1b5a9ea761653","hash":"414f04bd69c5c1fcca05bd386cf53ee66b3e91c6b8a2d66e75cd9f529060cb66","version":2,"size":370,"vsize":208,"weight":832,"locktime":916473,"vin":[{"txid":"2978c1c2eb9cf7a60830eaa8277ed8e9deee1ac2daa647a3c82929f45b8f8b19","vout":1,"scriptSig":{"asm":"","hex":""},"txinwitness":["30440220769f54fb918c1395c15f525464d2a8beac95bc7beabf4e960d0c00497587da650220315560714e4d108b9fa638036ed540ad0259e346e012b007155f3811c448146501","03d8b3473dc792dcbbbba76017d489b1aa72c37931c517571f8620c3b0ebfacc2e"],"sequence":4294967293},{"txid":"3e9d2a9e07901a3e29d4f7093bc9f75b6274cfef4ac100a299cbea9b94fbf2cd","vout":0,"scriptSig":{"asm":"","hex":""},"txinwitness":["304402204fbcc9592cba03f5dca49f266e1206fc860011f54e908c108a27c0edd25892d6022067e7d8699ea59fe03d87808d94df075c1d664efe10d888571f6d3bedf7e0f89101","025149d05380e6655e86762aa82b20de6ae46850cda0a3c8a1005ed098e2dbcafb"],"sequence":4294967293}],"vout":[{"value":0.00226419,"n":0,"scriptPubKey":{"asm":"0 0c3ce0578042786be1a4d6dc9456bd810813fa65","desc":"addr(bc1qps7wq4uqgfuxhcdy6mwfg44asyyp87n9akmrjl)#xh3tjcul","hex":"00140c3ce0578042786be1a4d6dc9456bd810813fa65","address":"bc1qps7wq4uqgfuxhcdy6mwfg44asyyp87n9akmrjl","type":"witness_v0_keyhash"}},{"value":0.00360964,"n":1,"scriptPubKey":{"asm":"0 1bcc8aa3401726a6a935307bb62d0cd003544245","desc":"addr(bc1qr0xg4g6qzun2d2f4xpamvtgv6qp4gsj9kydn2q)#6f3n9ppv","hex":"00141bcc8aa3401726a6a935307bb62d0cd003544245","address":"bc1qr0xg4g6qzun2d2f4xpamvtgv6qp4gsj9kydn2q","type":"witness_v0_keyhash"}}],"hex":"02000000000102198b8f5bf42929c8a347a6dac21aeedee9d87e27a8ea3008a6f79cebc2c178290100000000fdffffffcdf2fb949beacb99a200c14aefcf74625bf7c93b09f7d4293e1a90079e2a9d3e0000000000fdffffff0273740300000000001600140c3ce0578042786be1a4d6dc9456bd810813fa6504820500000000001600141bcc8aa3401726a6a935307bb62d0cd003544245024730440220769f54fb918c1395c15f525464d2a8beac95bc7beabf4e960d0c00497587da650220315560714e4d108b9fa638036ed540ad0259e346e012b007155f3811c4481465012103d8b3473dc792dcbbbba76017d489b1aa72c37931c517571f8620c3b0ebfacc2e0247304402204fbcc9592cba03f5dca49f266e1206fc860011f54e908c108a27c0edd25892d6022067e7d8699ea59fe03d87808d94df075c1d664efe10d888571f6d3bedf7e0f8910121025149d05380e6655e86762aa82b20de6ae46850cda0a3c8a1005ed098e2dbcafbf9fb0d00","blockhash":"0000000000000000000106d25732035975d14a5a0bf37a2a63f160bcfccf2a21","confirmations":6,"time":1758897247,"blocktime":1758897247},"id":1}`,
		// getblockheader
		`{"jsonrpc":"2.0","result":{"hash":"000000000000000000005223837ff93667f39a926a583a03bfedeee8f9da59af","confirmations":3,"height":916479,"version":706306048,"versionHex":"2a196000","merkleroot":"72c907e63ba41e561b2aeee89e48ded880422fb68d4020a527f20dd4f8c26614","time":1758899790,"mediantime":1758896752,"nonce":3092411906,"bits":"1701fa38","target":"00000000000000000001fa380000000000000000000000000000000000000000","difficulty":142342602928674.9,"chainwork":"0000000000000000000000000000000000000000e62215971010a8d2ab9c3320","nTx":3290,"previousblockhash":"000000000000000000017c484d8d350e26b4999719bf8334c22ebdbed6de43f5","nextblockhash":"000000000000000000003de597b30b9216bc3b605ec29752bca909ab214b6303"},"id":1}`,
		// getbestblockhash
		`{"jsonrpc":"2.0","result":"000000000000000000011734882627e0149787de4d66a864254a3d63529586ee","id":1}`,
		// getblockheader
		`{"jsonrpc":"2.0","result":{"hash":"000000000000000000011734882627e0149787de4d66a864254a3d63529586ee","confirmations":1,"height":916481,"version":537337856,"versionHex":"20072000","merkleroot":"0d321497ceb7eb229a1e37fd1280908dc4ed0dec07693561b80e5bcd36397e2f","time":1758900536,"mediantime":1758897247,"nonce":2935948034,"bits":"1701fa38","target":"00000000000000000001fa380000000000000000000000000000000000000000","difficulty":142342602928674.9,"chainwork":"0000000000000000000000000000000000000000e6231883838e80965d49ccf0","nTx":3511,"previousblockhash":"000000000000000000003de597b30b9216bc3b605ec29752bca909ab214b6303"},"id":1}`,
	}, 200)
	defer close()
	asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("mainnet").WithProvider(string(bitcoin.Blockbook)).WithDecimals(8)
	client, err := bitcoin.NewClient(asset)
	require.NoError(err)
	info, err := client.FetchLegacyTxInfo(s.Ctx, xc.TxHash("d802946fc7180d899e3006fd9b6991b0f42e2f4af9aec697ce811da068105d2e"))
	require.NotNil(info)
	require.NoError(err)
	require.EqualValues("d802946fc7180d899e3006fd9b6991b0f42e2f4af9aec697ce811da068105d2e", info.TxID)
	require.Len(info.Sources, 1)
	require.EqualValues("bc1qps7wq4uqgfuxhcdy6mwfg44asyyp87n9akmrjl", info.Sources[0].Address)
	require.EqualValues(226419, info.Sources[0].Amount.Uint64())

	require.Len(info.Destinations, 2)
	require.EqualValues("bc1q38mrfctfzw4jc86ju64594ltpy8lcv8zyd6fxu", info.Destinations[0].Address)
	require.EqualValues("bc1q668wvrnx0k5vavy7r96g4zuewnv6zjktzmpel0", info.Destinations[1].Address)
	require.EqualValues(61440, info.Destinations[0].Amount.Uint64())
	require.EqualValues(164270, info.Destinations[1].Amount.Uint64())
	require.EqualValues(xc.TxStatusSuccess, info.Status)
	require.EqualValues(3, info.Confirmations)
	require.EqualValues(709, info.Fee.Uint64())
}

func (s *ClientTestSuite) TestNotFoundFetchTxInfo() {
	require := s.Require()

	type testcase struct {
		resp []string
	}

	testcases := []testcase{
		{
			resp: []string{
				`{"jsonrpc":"2.0","error":{"code":-5,"message":"No such mempool or blockchain transaction. Use gettransaction for wallet transactions."},"id":1}`,
			},
		},
	}
	for _, v := range testcases {
		server, close := testtypes.MockHTTP(s.T(), v.resp, 400)
		defer close()
		asset := xc.NewChainConfig("BTC").WithUrl(server.URL).WithNet("testnet").WithProvider(string(bitcoin.Blockbook))
		client, err := bitcoin.NewClient(asset)
		require.NoError(err)
		_, err = client.FetchLegacyTxInfo(s.Ctx, xc.TxHash("227178d784150211e8ea5a586ee75bc97655e61f02bc8c07557e475cfecea3cd"))
		require.Error(err)
		require.ErrorContains(err, "TransactionNotFound:")
	}

}
