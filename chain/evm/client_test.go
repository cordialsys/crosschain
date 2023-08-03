package evm

import (
	"errors"
	"fmt"

	xc "github.com/jumpcrypto/crosschain"
	testtypes "github.com/jumpcrypto/crosschain/testutil/types"
)

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()
	client, err := NewClient(&xc.NativeAssetConfig{})
	require.NotNil(client)
	require.False(client.Legacy)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestNewLegacyClient() {
	require := s.Require()
	client, err := NewLegacyClient(&xc.NativeAssetConfig{})
	require.NotNil(client)
	require.True(client.Legacy)
	require.Nil(err)
}

func (s *CrosschainTestSuite) TestLegacyGasCalculation() {
	require := s.Require()

	// Should sum 1000 + 200
	estimate := EvmGasEstimation{
		BaseFee:    xc.NewAmountBlockchainFromUint64(1000),
		GasTipCap:  xc.NewAmountBlockchainFromUint64(200),
		Multiplier: 1.0,
	}

	// Multiplier should default to 1
	require.EqualValues(1200, estimate.MultipliedLegacyGasPrice().Uint64())
	estimate = EvmGasEstimation{
		BaseFee:    xc.NewAmountBlockchainFromUint64(1000),
		GasTipCap:  xc.NewAmountBlockchainFromUint64(200),
		Multiplier: 0.0,
	}
	require.EqualValues(1200, estimate.MultipliedLegacyGasPrice().Uint64())

	// 1.5x
	estimate = EvmGasEstimation{
		BaseFee:    xc.NewAmountBlockchainFromUint64(1000),
		GasTipCap:  xc.NewAmountBlockchainFromUint64(200),
		Multiplier: 1.5,
	}
	require.EqualValues(1800, estimate.MultipliedLegacyGasPrice().Uint64())
	// 0.5x
	estimate = EvmGasEstimation{
		BaseFee:    xc.NewAmountBlockchainFromUint64(1000),
		GasTipCap:  xc.NewAmountBlockchainFromUint64(200),
		Multiplier: 0.5,
	}
	require.EqualValues(600, estimate.MultipliedLegacyGasPrice().Uint64())
}

func (s *CrosschainTestSuite) TestAccountBalance() {
	require := s.Require()

	vectors := []struct {
		resp interface{}
		val  string
		err  string
	}{
		{
			`"0x123"`,
			"291",
			"",
		},
		{
			`null`,
			"0",
			"cannot unmarshal non-string into Go value of type",
		},
		{
			`{}`,
			"0",
			"cannot unmarshal non-string into Go value of type",
		},
		{
			errors.New(`{"message": "custom RPC error", "code": 123}`),
			"",
			"custom RPC error",
		},
	}

	for _, v := range vectors {
		server, close := testtypes.MockJSONRPC(&s.Suite, v.resp)
		defer close()

		client, _ := NewClient(&xc.NativeAssetConfig{URL: server.URL, Type: xc.AssetTypeNative})
		from := xc.Address("0x0eC9f48533bb2A03F53F341EF5cc1B057892B10B")
		balance, err := client.FetchBalance(s.Ctx, from)

		if v.err != "" {
			require.Equal("0", balance.String())
			require.ErrorContains(err, v.err)
		} else {
			require.Nil(err)
			require.NotNil(balance)
			require.Equal(v.val, balance.String())
		}
	}
}

// func (s *CrosschainTestSuite) TestFetchTxInput() {
// 	require := s.Require()
// 	client, _ := NewClient(xc.AssetConfig{})
// 	from := xc.Address("from")
// 	input, err := client.FetchTxInput(s.Ctx, from, "")
// 	require.NotNil(input)
// 	require.EqualError(err, "not implemented")
// }

func (s *CrosschainTestSuite) TestFetchTxInput() {
	require := s.Require()

	vectors := []struct {
		name       string
		resp       interface{}
		val        *TxInput
		err        string
		multiplier float64
		legacy     bool
	}{
		// Send ether normal tx
		{
			name: "fetchTxInput normal",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_getBlockByNumber
				`{"jsonrpc":"2.0","id":2,"result":{"baseFeePerGas":"0xba43b7400","difficulty":"0x19","extraData":"0xd682040083626f7288676f312e31392e37856c696e7578000000000000000000ec6ff50a6a950f50eb734ee68765c988ce7dbaae7af3f92f5604100affceb6d87fd6ad01972752001d4e067ec14482630bb88db995f5503c76cfd625c949922300","gasLimit":"0x1c0e7cb","gasUsed":"0x0","hash":"0x32c7587e0c0634a19c40dee211323dd0b2d83494f65d619a9ddefa6d31f99238","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","miner":"0x0000000000000000000000000000000000000000","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","number":"0x2bbd145","parentHash":"0x6c63c167c9014fb62dad62dde72f774f64634237d18f5877ec3642c44b3af2dd","receiptsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x269","stateRoot":"0x70d3c1f93205f1b6970b6e0ebc5a20c938dbcc8050c82e07a09fa1d568a9d428","timestamp":"0x64cbc1e1","totalDifficulty":"0x2f15808f","transactions":[],"transactionsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","uncles":[]}}`,
				// eth_maxPriorityFeePerGas
				`"0x6fc23ac00"`,
			},
			val: &TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVM),
				Nonce:           6,
				GasLimit:        0,
				GasFeeCap:       xc.NewAmountBlockchainFromUint64(50000000000),
				GasTipCap:       xc.NewAmountBlockchainFromUint64(30000000000),
				// legacy price
				GasPrice: xc.NewAmountBlockchainFromUint64(50000000000 + 30000000000),
			},
			err:        "",
			multiplier: 1.0,
		},
		{
			name: "fetchTxInput normal 2x",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_getBlockByNumber
				`{"jsonrpc":"2.0","id":2,"result":{"baseFeePerGas":"0x14f46b0400","difficulty":"0x19","extraData":"0xd682040083626f7288676f312e31392e37856c696e7578000000000000000000ec6ff50a6a950f50eb734ee68765c988ce7dbaae7af3f92f5604100affceb6d87fd6ad01972752001d4e067ec14482630bb88db995f5503c76cfd625c949922300","gasLimit":"0x1c0e7cb","gasUsed":"0x0","hash":"0x32c7587e0c0634a19c40dee211323dd0b2d83494f65d619a9ddefa6d31f99238","logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","miner":"0x0000000000000000000000000000000000000000","mixHash":"0x0000000000000000000000000000000000000000000000000000000000000000","nonce":"0x0000000000000000","number":"0x2bbd145","parentHash":"0x6c63c167c9014fb62dad62dde72f774f64634237d18f5877ec3642c44b3af2dd","receiptsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x269","stateRoot":"0x70d3c1f93205f1b6970b6e0ebc5a20c938dbcc8050c82e07a09fa1d568a9d428","timestamp":"0x64cbc1e1","totalDifficulty":"0x2f15808f","transactions":[],"transactionsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","uncles":[]}}`,
				// eth_maxPriorityFeePerGas
				`"0x77359400"`,
			},
			val: &TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVM),
				Nonce:           6,
				GasLimit:        0,
				GasFeeCap:       xc.NewAmountBlockchainFromUint64(90000000000 * 2),
				// GasTip should not get multiplied
				GasTipCap: xc.NewAmountBlockchainFromUint64(2000000000),
				// legacy price
				GasPrice: xc.NewAmountBlockchainFromUint64((90000000000 + 2000000000) * 2),
			},
			err:        "",
			multiplier: 2.0,
		},
		{
			name: "fetchTxInput legacy",
			resp: []string{
				// eth_getTransactionCount
				`"0x6"`,
				// eth_gasPrice
				`"0xba43b7400"`,
			},
			val: &TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEVM),
				Nonce:           6,
				GasLimit:        0,
				GasFeeCap:       xc.NewAmountBlockchainFromUint64(50000000000),
				GasTipCap:       xc.NewAmountBlockchainFromUint64(0),
				// legacy price
				GasPrice: xc.NewAmountBlockchainFromUint64(50000000000),
			},
			err:        "",
			multiplier: 1.0,
			legacy:     true,
		},
	}
	for _, v := range vectors {
		fmt.Println("testing ", v.name)
		server, close := testtypes.MockJSONRPC(&s.Suite, v.resp)
		defer close()
		asset := &xc.NativeAssetConfig{NativeAsset: xc.ETH, URL: server.URL, ChainGasMultiplier: v.multiplier}
		client, err := NewClient(asset)
		if v.legacy {
			client, err = NewLegacyClient(asset)
		}
		require.NoError(err)
		input, err := client.FetchTxInput(s.Ctx, xc.Address(""), xc.Address(""))
		require.NoError(err)
		if v.err != "" {
			require.Equal(TxInput{}, input)
			require.ErrorContains(err, v.err)
		} else {
			require.Nil(err)
			require.NotNil(input)
			require.Equal(v.val, input)
		}
	}
}

func (s *CrosschainTestSuite) TestFetchTxInfo() {
	require := s.Require()

	vectors := []struct {
		name   string
		txHash string
		resp   interface{}
		val    xc.TxInfo
		err    string
	}{
		// Send ether normal tx
		{
			"normal_ether_deposit",
			"0xbca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0",
			[]string{
				// eth_getTransactionByHash
				`{"blockHash":"0xd090a9e97e00aa135710a92c827def07e4c8ff2269fd69411c48402e0a6a2a89","blockNumber":"0x8914cc","from":"0x17519be39a6b67a19468dfbdc1d795c38232c274","gas":"0x5208","gasPrice":"0x3e22ba6cde5","hash":"0xbca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0","input":"0x","nonce":"0x1","to":"0xa0a5c02f0371ccc142ad5ad170c291c86c3e6379","transactionIndex":"0x0","value":"0x49da27372c2bdee6","type":"0x0","chainId":"0x5","v":"0x2d","r":"0xbc8fc6daecc690912d8e6b7ab4e47188b562a2d5094a04b23b116753a077099a","s":"0x5ae2837a3b5b6cdf88c0cd367a6faaa52f38d7f71bc4f871dba389ad18237015"}`,
				// eth_getTransactionReceipt
				`{"blockHash":"0xd090a9e97e00aa135710a92c827def07e4c8ff2269fd69411c48402e0a6a2a89","blockNumber":"0x8914cc","contractAddress":null,"cumulativeGasUsed":"0x5208","effectiveGasPrice":"0x3e22ba6cde5","from":"0x17519be39a6b67a19468dfbdc1d795c38232c274","gasUsed":"0x5208","logs":[],"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","status":"0x1","to":"0xa0a5c02f0371ccc142ad5ad170c291c86c3e6379","transactionHash":"0xbca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0","transactionIndex":"0x0","type":"0x0"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xb","difficulty":"0x0","extraData":"0xd883010b04846765746888676f312e32302e32856c696e7578","gasLimit":"0x1c9c380","gasUsed":"0x27e6d9","hash":"0xd090a9e97e00aa135710a92c827def07e4c8ff2269fd69411c48402e0a6a2a89","logsBloom":"0x44000000002040440011000200010000100008000040008800000000800000000122000108000000001400080004008404001004400002200008140400200000000240019800000040000008000210000005000000008000004001000008002000002200020200000000008100100820200004400420458808c242140080102093020050080010200000800000000000000000000010028004030001104200000600000000100200000201000000020000000200a04c0340008000060020000280300003000000080002204000020000000090000080028020010040080020000010000004200004201001200040000800400008029002000200401800000002","miner":"0x0000000000000000000000000000000000000000","mixHash":"0xb460ee4e35822216ac484dfcb7641fef4b9afed393279b13ec4faeade6bbce99","nonce":"0x0000000000000000","number":"0x8914cc","parentHash":"0xc6c2e8a0f3395d584ad2bcff333736a9dd7245d9a223a4e4bc252e32b442c610","receiptsRoot":"0xcc3d1989ea341f5f696ad76b34b60971687ab688f573eecd5e50302d140021e5","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x27da","stateRoot":"0x6401e2e073ac8a27bb99b42151333f172c8aef69c54d33805e463c3deafde36b","timestamp":"0x645d6cb0","totalDifficulty":"0xa4a470","transactions":["0xbca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0","0xbce5302a41d8b84f7dfc9f987a47d9b6711d7f84e939e0acd0f0623d2db5275a","0xd2a1a4b08185e707a388a78af90c253fbbcc249b1b24594df42dcf270c90b94f","0xef4c32a8fe320b54017d462ea5759ef3fa3d8f170775d22639d01ee431104b53","0x3e9cdbc9782bb7042198096ea4fa6c2b07c0e54a0325ae9bf3443c0be57d5917","0x76461b1ad7b8b0b586b2e9a311cda9127385e0c43366350e3fa9e8b9af1cf8df","0xcd1c83f194e0fe6f7e3da8285e7da31bdd002d150af765370212997270271488","0x3331263d8b0e068fe7a89583deddcec20152375766a1a07010b11184932aaf79","0x998b5339862b5a9cd10936b5ea73236a04f148d10ffa5ecc5122128fc957ed02","0xfd212e955146ded9ea2416fd65eb7c23ace7d7175e00d97139a8c415c60575ef","0xb5bf8cec91d0ba6026a01b0b933d6f6912e79595e0d65a92c07b313ae7c6af63","0x554a1ddc07ed336b40b1c561f76ebd5bed6dc75cdd9562fab250a36372de61d3","0x35837da336ecb064bc9348d504a943b0b4d36b51dc1d44343ca1fd30964fb253","0x3a24aaafed98a979b03c3c8057ad7fb96209cadc2be89304a4671a817fd719fc","0x39e459b2504df5ac04a220aa33ffeb24257c01c5c006d244f3d1ae4df5e0bcb9","0xb8b01c71bcea7008b666c6146ca807fc3737d6dfa0de882e1bf041d8f9a5d582","0xde2fdbecf03afc07e202a37f92a439d58760ce091b008ae18f1edb2ea15d2073"],"transactionsRoot":"0x69f8755536692771ea8dcfc81dd31df84fe3829c59aa6ae8b46d5de3ebf944aa","uncles":[],"withdrawals":[{"index":"0x4fcf74","validatorIndex":"0x9d40","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1eeecf"},{"index":"0x4fcf75","validatorIndex":"0x9d41","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e3848"},{"index":"0x4fcf76","validatorIndex":"0x9d42","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e686f"},{"index":"0x4fcf77","validatorIndex":"0x9d43","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1d40a9"},{"index":"0x4fcf78","validatorIndex":"0x9d44","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1f459d"},{"index":"0x4fcf79","validatorIndex":"0x9d45","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e01e6"},{"index":"0x4fcf7a","validatorIndex":"0x9d46","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1ed357"},{"index":"0x4fcf7b","validatorIndex":"0x9d47","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e24b1"},{"index":"0x4fcf7c","validatorIndex":"0x9d48","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e17ee"},{"index":"0x4fcf7d","validatorIndex":"0x9d49","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e344f"},{"index":"0x4fcf7e","validatorIndex":"0x9d4a","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e0d01"},{"index":"0x4fcf7f","validatorIndex":"0x9d4b","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1dfe14"},{"index":"0x4fcf80","validatorIndex":"0x9d4c","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e47c6"},{"index":"0x4fcf81","validatorIndex":"0x9d4d","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e76dd"},{"index":"0x4fcf82","validatorIndex":"0x9d4e","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e0202"},{"index":"0x4fcf83","validatorIndex":"0x9d4f","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1d032a"}],"withdrawalsRoot":"0x401484fc49fe77cb4d010977c5d26f6621c9b586377a3572b9c542dc996267fb"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xc","difficulty":"0x0","extraData":"0x506f776572656420627920626c6f58726f757465","gasLimit":"0x1c9c380","gasUsed":"0x6370e2","hash":"0x6731d229b8ee277afb29ef9f0505aa628642842e6ba9ba484ec26f3d74cb5e14","logsBloom":"0x00200800024010000018000280010080000d0000014000080045001088000010012210010001220200000001010400840000100440020304040016048024004000060000047020804900000b00a002240008c2000500300180088000a08800200080310002000000000000a40010084080000804010000000124041488000000410222200000100a2440001080000402000044810004008800800145102200400200000010000010000000020010000000400200204c00400100014000a20002006040020180000885422001050240080048000000814010000122000aa2a10a28100229340042008400436020002800000111000a0040d00000400000000041","miner":"0x8dc847af872947ac18d5d63fa646eb65d4d99560","mixHash":"0x0fcc735f658e8a7a25ae8d8230a683f9408a1d9bc0e9b3526b0c5c3807d492c6","nonce":"0x0000000000000000","number":"0x8914ec","parentHash":"0x705a0b921245d4e480933e5fd1cab446ded464600b374aaa21bb9d2fbf6dd071","receiptsRoot":"0xc3853474f16acbc611eb5ca71dd8c54ace3100d878363c57a882ae8378955988","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x16b26","stateRoot":"0x26f735fceea3cfb2c72749da246e7f53934a6585f4727e1b0709d5c74995807f","timestamp":"0x645d6e84","totalDifficulty":"0xa4a470","transactions":["0x638e9ead7eaddf7a8ad32f1729f4a6991815f8c04846b92245557370a52cdafc","0x52726e2b57c4a32e29f0ac293207f25d77875595a4ee6b181adfb06146b0fffe","0xe93306de0a0c500013e05c6ffb7c501c6def1045fb8a559796b953c1e466260d","0x4dd95cc5c4fe7aa435643ae51248cee90ded85ad992ac99f8079957806c203d3","0x308553ae908896a3a78cc4bce5e9fa32b8449213a9a8003bdad3e2daa09076b5","0x47364731e2320f460d9272c13c76fd9dce80e91fda3b8ce80fe08a64086d59e3","0xc6f77048dfa354af3f77d69245dc62106af2738d33d4f5f8b468141f897b148f","0xdbad6020b3387db9f0cb1e464bf1ec765526d5fbeed0d780bb70ce5495f72303","0xbde8d289ad2a3d90e5678acef02b3b042b61626be0a39ed639b0a38879d0a8e1","0x884a4f9d6e3af9fd2f5c72b93b903dc1792df52001e12b114dc6bf44e01c4ead","0x662ed8fa609854f586e3f4eafdec3d55f5531499b7a23bd80644e79ed6b2b053","0x69026737fc70002e523e7703ce1e0d89af7390d529d308cb5e08e2c55c3fc1e1","0x542890dad9bea30d477893415eda3d8f3f2a3bf18e709f3f4e24c167d5dd10dc","0xdd33d366802312281f4f872097805e519ae4617733fcd978b22d1f7e1abbb7d9","0x79dfc706d7581c697ebe08d540894daaca00d496b8ba3e6c49bcf800e7a0aa22","0x4b90fc9584f3084f82c75dc676830e87b3669f9773f7611a64562683bcf09f92","0x482a3358d2a18c264e73e1e11036241f50554a78b854b35c6d1adfc2d1d4c218","0x2e8ffa43bfc5a9916f8eb469e343b841fa4bd56ff388b4194e9cf9be9133bc9f","0x5faeaf922157d605c19f8a5a75f449960ded61297a50a7ed0afc887c209bcab1","0x935c2426af0ce34c53d7a9c074b3c6182740c943612315f08269a0cc010055e7","0x04ff942582a2504fb39a6be111f9db97d9827d27fd0c2841cc7961935fd1be6c","0xa46de5b9bbf3848ca4572cc45c4c2d694a9c7f2d45edaea75f492342f0715226","0xad77b0c03769ceeb226f41ab8edc4e202b14b3efc1c83a7e3eb78bc3599c7dbe","0x82bdfff652050b14a56dfc839b7d160114976ccb3da037f3c272d4b1aa101439","0x2ebfffde39cdc03beaf4bccc62c3a08a351fbd1f62303a7aad437ce696794e45","0xea05481bc288c83aa859e3ee29f838ec69e145f5dbd5d9ca5d3c4ae9b281cd80","0xf3fe1622f73fb60d92794266d980bbe9a35abf475a5e25d36a60773a1b9ca09a","0x5b349c397c4a5635a6b3804e375c0682835fbba2e0d705310b1c64d258c49dab","0xc7b07bdd5677c3c346f49bd2d54d65370c34a9dd8c29ea6d4bb1729ab6e13639"],"transactionsRoot":"0x73be4e736e029b9fdcbda4813ad4731eb0b394f37577f150e637a5a40926ca2f","uncles":[],"withdrawals":[{"index":"0x4fd174","validatorIndex":"0x9f40","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1d48af"},{"index":"0x4fd175","validatorIndex":"0x9f41","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e3c54"},{"index":"0x4fd176","validatorIndex":"0x9f42","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1eccb3"},{"index":"0x4fd177","validatorIndex":"0x9f43","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1ed7a0"},{"index":"0x4fd178","validatorIndex":"0x9f44","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e48b5"},{"index":"0x4fd179","validatorIndex":"0x9f45","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1db4e6"},{"index":"0x4fd17a","validatorIndex":"0x9f46","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1a460a7"},{"index":"0x4fd17b","validatorIndex":"0x9f47","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1ee461"},{"index":"0x4fd17c","validatorIndex":"0x9f48","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e3be4"},{"index":"0x4fd17d","validatorIndex":"0x9f49","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e9651"},{"index":"0x4fd17e","validatorIndex":"0x9f4a","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1defd9"},{"index":"0x4fd17f","validatorIndex":"0x9f4b","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1e8c12"},{"index":"0x4fd180","validatorIndex":"0x9f4c","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1dd8c7"},{"index":"0x4fd181","validatorIndex":"0x9f4d","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1ec5c4"},{"index":"0x4fd182","validatorIndex":"0x9f4e","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1df604"},{"index":"0x4fd183","validatorIndex":"0x9f4f","address":"0xf36f155486299ecaff2d4f5160ed5114c1f66000","amount":"0x1ec6f6"}],"withdrawalsRoot":"0x5579318dff536da3ea63a244c27453e29654b9c97a235ca1c00934663fe1f26c"}`,
			},
			xc.TxInfo{
				BlockHash:     "0xd090a9e97e00aa135710a92c827def07e4c8ff2269fd69411c48402e0a6a2a89",
				TxID:          "bca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0",
				ExplorerURL:   "/tx/0xbca068cf854af49fc6b28ff5405068d51d3cefb870e624d22d046005a22349d0",
				From:          "0x17519Be39A6B67a19468dfbDc1D795c38232c274",
				To:            "0xa0a5C02F0371cCc142ad5AD170C291c86c3E6379",
				BlockIndex:    8983756,
				BlockTime:     1683844272,
				Confirmations: 32,
				Sources: []*xc.TxInfoEndpoint{
					{
						Address:     "0x17519Be39A6B67a19468dfbDc1D795c38232c274",
						Amount:      xc.NewAmountBlockchainFromStr("5321609027609419494"),
						NativeAsset: "ETH",
					},
				},
				Destinations: []*xc.TxInfoEndpoint{
					{
						Address:     "0xa0a5C02F0371cCc142ad5AD170C291c86c3E6379",
						Amount:      xc.NewAmountBlockchainFromStr("5321609027609419494"),
						NativeAsset: "ETH",
					},
				},
				Fee:    xc.NewAmountBlockchainFromStr("89668526728137000"),
				Amount: xc.NewAmountBlockchainFromStr("5321609027609419494"),
			},
			"",
		},
		// Send erc20 normal tx
		{
			"normal_erc20_deposit",
			"0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b",
			[]string{
				// eth_getTransactionByHash
				`{"blockHash":"0x03d633a561d2217f8d7ae529ed90f3f4709fd62a5fb1b0ff6f7ce487f2113ba7","blockNumber":"0x891528","from":"0xe8be958f910fb1bb439eafbcfd0475509ab6d43f","gas":"0x8757","gasPrice":"0x59682f0b","maxPriorityFeePerGas":"0x59682f00","maxFeePerGas":"0x59682f0d","hash":"0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b","input":"0xa9059cbb0000000000000000000000005d2ebdf613d50dc598a09d8ebdc3f285be6cf8ed000000000000000000000000000000000000000000000000000009184e72a000","nonce":"0x33","to":"0xb4fbf271143f4fbf7b91a5ded31805e42b2208d6","transactionIndex":"0xe","value":"0x0","type":"0x2","accessList":[],"chainId":"0x5","v":"0x0","r":"0xabf9ed0aa2255eda67c71e8fed5ee44b50059d507216221ea81400fa2db82289","s":"0x69e16a358d7a216499825143fdfe71f0c7d31d2a5613c5f0aeb5cdff7f04d39d"}`,
				// eth_getTransactionReceipt
				`{"blockHash":"0x03d633a561d2217f8d7ae529ed90f3f4709fd62a5fb1b0ff6f7ce487f2113ba7","blockNumber":"0x891528","contractAddress":null,"cumulativeGasUsed":"0x2f3b78","effectiveGasPrice":"0x59682f0b","from":"0xe8be958f910fb1bb439eafbcfd0475509ab6d43f","gasUsed":"0x8757","logs":[{"address":"0xb4fbf271143f4fbf7b91a5ded31805e42b2208d6","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000e8be958f910fb1bb439eafbcfd0475509ab6d43f","0x0000000000000000000000005d2ebdf613d50dc598a09d8ebdc3f285be6cf8ed"],"data":"0x000000000000000000000000000000000000000000000000000009184e72a000","blockNumber":"0x891528","transactionHash":"0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b","transactionIndex":"0xe","blockHash":"0x03d633a561d2217f8d7ae529ed90f3f4709fd62a5fb1b0ff6f7ce487f2113ba7","logIndex":"0x18","removed":false}],"logsBloom":"0x00000000000000000000000000000000000000000000000000000400000000000000100000000000000000000000000000000000000000000000020000000000000040000400000000000008000000000000400000000000000000000000000000000000000000000000100000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004202000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","status":"0x1","to":"0xb4fbf271143f4fbf7b91a5ded31805e42b2208d6","transactionHash":"0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b","transactionIndex":"0xe","type":"0x2"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xb","difficulty":"0x0","extraData":"0xd883010b06846765746888676f312e32302e33856c696e7578","gasLimit":"0x1c9c380","gasUsed":"0x3edb5d","hash":"0x03d633a561d2217f8d7ae529ed90f3f4709fd62a5fb1b0ff6f7ce487f2113ba7","logsBloom":"0xc00c04400500404c00110000840900841002080000000008810004128011008004221400000000000020000801840084440010144002002000001600002022400882402294400000500000090022120000044040010a204280408100000800010000020002000000904090a52410081000000040000841c808a2581400001000130030e0b000002a00000800000004000000048080100280000901010046008046000004300000004810000228000200200002012040024003020104000200029005462208200008040320410502440004489400000800000103a000080221001030002020200004a00001200000600000000200001002000002080002000000","miner":"0x9427a30991170f917d7b83def6e44d26577871ed","mixHash":"0xbe979c23ebac83783f28bd19bc54156afede8a9881f67ee68e585c617d35fa8c","nonce":"0x0000000000000000","number":"0x891528","parentHash":"0x4c0741acfa16a7544e8384ae6ff8e99fc13f7d4f425d37e9b98d649c205d3a7f","receiptsRoot":"0x1b208b79d2ac75825098ae78411aefb6e2e9a475fad97d8d1a1daa8642bde2cd","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0xb581","stateRoot":"0xdffaf71a94e6b64d04bc9b5d18a07f1c38c85038504d8ac898c81249e7826578","timestamp":"0x645d71cc","totalDifficulty":"0xa4a470","transactions":["0x2a02b2fea90d4d2210c2b106bfa09bd3083329285479ea56cbb459d1c1d04a39","0xb12cbda3b2674432ce747a93d6297bb153fe8934c34d49c7c462fe251a805552","0x231c2aa257987ceb68d837e5b6625d9783cec0b6eecedd7f35851096ef3a064e","0xefe67c0b5fde1610393b5e81baaf6be946d40c1a8b5cc35e3e96913bc55a08f4","0xefd5d8f84bad43612d72949db1a031a2e631bdfa31ba4da0b56737ec42c0fc34","0x68a3b34a9c88b646b47f9349ad87093fc7696be1d8607eaba41ae60458ed0b00","0x2a7003970d1352ef5b3f36b8e4567f0f79b12c1221aca6def7541c714bd63275","0xefd4a264368a75f0aadd042f1937623cea77a424b32afe03b40bd71fa0cd3c58","0xc7cf43c1b7382901b229f43137b614c86676710863f477b1b976e76a0965095f","0x0a24277f4c37e23a3eb6b1aa5690dea691b86d21cdd8e9b4a63d43c5bc5f358b","0xc769cfe9366d3cc102971db20ee15d77bf22eaedb720a5ab74e985d730129cfe","0x96cf02bdac52cf94bb32192a6ba717cd54832ba182f9bbe6e2fc08674ec3ae4a","0x8dcb26141ec67dcba395a7d07ff4cf2aa7c6faff9b55f384705b131fef5ea127","0x8915c01b564b7ca7f8c3bc8b20cf7311d94e8570c6bdff196b55bb2751ef70d1","0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b","0x79a7dc020d3d6989de12e22f7191f9338a96693288fc4fbeaed1bdae7173f46e","0x5ed535730523c3eb3db632b2d59579004bc01a30cd1a40b4e52b94ca07606902","0xd7ce3cae178416095b0205d987d225519420cd93bbad595670e8794544177db5","0x4233c7fea875b11ba216fb98e65ef3fe53b47ab0d07aef3debfaf7c6a14c9d72","0x76b67bfdf76b4242e3d503bec804feed60fc6edd65d57c0a09beef0102db3b70","0x4b5f4b515f0c7e596b959faa233f76402aa70131b0469032e21fe2160e10e2ca"],"transactionsRoot":"0x70a7dbc37ea27140077f216ce19070150afbfbc1abf758d085c9021b463667de","uncles":[],"withdrawals":[{"index":"0x4fd4ac","validatorIndex":"0x33979","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x19f56d"},{"index":"0x4fd4ad","validatorIndex":"0x3397a","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1ca348"},{"index":"0x4fd4ae","validatorIndex":"0x3397b","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1cc394"},{"index":"0x4fd4af","validatorIndex":"0x3397c","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1a6016"},{"index":"0x4fd4b0","validatorIndex":"0x3397d","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x186780"},{"index":"0x4fd4b1","validatorIndex":"0x3397e","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c38d5"},{"index":"0x4fd4b2","validatorIndex":"0x3397f","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x34f8fd0"},{"index":"0x4fd4b3","validatorIndex":"0x33980","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1a04eb"},{"index":"0x4fd4b4","validatorIndex":"0x33981","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1a0e4c"},{"index":"0x4fd4b5","validatorIndex":"0x33982","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1d8c4a"},{"index":"0x4fd4b6","validatorIndex":"0x33983","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1cbb62"},{"index":"0x4fd4b7","validatorIndex":"0x33984","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1d1aac"},{"index":"0x4fd4b8","validatorIndex":"0x33985","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c8852"},{"index":"0x4fd4b9","validatorIndex":"0x33986","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x18c245"},{"index":"0x4fd4ba","validatorIndex":"0x33987","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c978f"},{"index":"0x4fd4bb","validatorIndex":"0x33988","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c835c"}],"withdrawalsRoot":"0x19c8289ae378446696ca072ea8bd762ba1ebd465e505e31ed93d89b99b62ae8c"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xa","difficulty":"0x0","extraData":"0xd883010b05846765746888676f312e32302e32856c696e7578","gasLimit":"0x1c9c380","gasUsed":"0xe7023b","hash":"0x7041515bf99b52262d4faeb17ba2a5f772ea463c30db9798371ff2094aa75ddc","logsBloom":"0x006408484088f02c801010009515008412eb29040002580c858101128851002004aa84e00080800041110000150401a4485110540020082012001624c124a26100026032947080805802020800801224020d82404007202080518105888b08010010350492478080804004a604102b0000080036000c50c48020125602001040132020621880ad0a20442c44400305264001800102100a880001014914668040660c000638880202101200222518830026000220a04402400702018408a2000390a144a3014020082402204345126444000814000080229030432739098261821810002120628114a854032c1280501001012300000812508106400802100080","miner":"0xc6e2459991bfe27cca6d86722f35da23a1e4cb97","mixHash":"0x522990debf63b7812eb8b6b96e0c56f91d567b8e2109e0a5f77450e7315ca21d","nonce":"0x0000000000000000","number":"0x89153b","parentHash":"0x41957451a5214e3420e12199c702016f2aa6a2d4848d429eefdfccc98661afd9","receiptsRoot":"0x01d943be32594c16c5c7558ca9b0fed2bfbb159e4e607835aeb22a827916f552","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0xfb37","stateRoot":"0x0d2abf67da6fd97a434c3d39058eff6acdf45b276e40b387eb32390c576fc413","timestamp":"0x645d72e0","totalDifficulty":"0xa4a470","transactions":["0x1aae2d96b2aee3ecfb01d95c33b32493df758a99b93046ff7dd50172b8cf6d30","0xa498121ad49d5dfc5b46d24b85ea857b426a1ed0cf4133c2cb96561a478d8e08","0xbd4de1c91760f3123402e74a6814fd972fa9a5c03fd18b13ffb7ec7586883425","0xce48fb1ee75beb40acee9bc54dda29e7811a4d3eed1d635ce7ba184979a6d883","0xe532067ac4b74deffca708c1eec3cfe7ba79dbac70518fb752d86941d828e4eb","0x7eb0a23f03525a0830befde8bfa3fa7c1bccef491711356c4880d401b62eb671","0xde51711ca74c63e8d9afdd8152d0a2c0c4f125ba3f6a9126c7d490fdd032c4cf","0xbd6aa59ce286c4221afd9000f834ddc207a93a7bc2a9ae068713ce8a43ee2742","0x746f05148b6ce98fc136d7c4ed00a73cc5c2ea68ab80bd3c5d3bd94402e20cc7","0x32991b9b8a1b1ba7ff7a9763f59e2ea1543acd28d6e2789d328a8317b4f74244","0x5a21a15f8736d513c48ce432ef3640c647f22d2ebb3dba833b0edfc92f27ef1d","0x4ee9cb1837031eb167b1357e8c9c19701ef6295729c2c580f9f7b9dee3b03f7c","0x4d5d016a4067cac522a426f12698d10915546fb65136074eb971cffbb8d6169e","0x68007016d532e43af452ef414a95040259ba86dcc26f380985673601e6692522","0xea0a297b661d014fedfdb2da3268fa1798db565d90ca9596bdf71df8b573df80","0x81058b8e6beef482f6a93e74c8cc5eaec892466465c3094713a950179c1a8785","0x786824ed4919c538506cfa100ee2e96869dca946d9e2be02401a8f7e1f66c6a8","0xdee6f95049d81f0d8375db35e5d0a668d8492032b1448a94b0f7912ec6c2ac64","0x61134262a45b0cdeb52d96e90c97780bc08779af8b086036d61d0657623dda01","0xbe825ab6ca9760557b9fa90e88c0d5f3fc58f5dd197bc2454f27d5e05b482f10","0xd71385e6984b41eafa8ef8e8614ec7c955e0c6c0073b2ae0073d4be11c1f7610","0x5e4ce86d4b6b09ef361f873a8ccb02c6b28f203b54a5a299ff5b3a486a5591e0","0x0ce8a4a79d39c3d8f57eb632deefd3901c23ae9cb9f49fc8811bc815195e7ec1","0xbd5f6f91610afd56a4d80483e38c14b7b479876e6f156828590c75bc09a317f4","0x824a8744c6cc856e49239d0e6547396d8435944bdb8fe89ca868f5a6c31a9c3f","0xf46646fc9e738b4079367c8d950b6037315ecff744b6fefa52afe1943c88a45e","0xb77420a96f2b4be4a4bedac65b41bdd98e5e7541edfa022451cfc927ec46c60f","0x5d4fa90d7fd3fe0571f403149cd88869f9185ce6854a323ed8d454e319f4d80a","0x6dbdd47cdaa9cd3c391c67d6d623e7bbeb3302c09ea7b4770ee0649245dfc80e","0x50e6add79c2895a3f78f748eff4778d52f5e428e2672e23fa38b9347559e451d","0xd0f55fd2c13714ba6aee6cff8851733545ce2a2b1a89723f88d039a0d68f68d8","0xd4965e1587ec7256dbdda89b73d961cd5d4d5623ddbe78bca33eb00de141f236","0x62ccd2c5e3b5bff5be54f95f5f26e70043edb8e3dc875ae4c7aa9ca7bb26f4fa","0xea90ddc11b18a323704f9b488f854e915ed1f85e47a5aa97d92490c3cb104f6b","0x23d2bd4b44bff5517b1302d15fa69195ddd288ec666cf7ac06c091ed415324d0","0xdc6d15f3e70d77a059570e8f797c8bd77bc1a763e03351175dcafe80235c8654","0xfba6bed9ee750c686a7f97fb1639a9eadd746ba47d367221caa9a3d6ca8e76b3","0x382510b2ee498a11dfc79c13ccc881221b5d22ac1079c42ece5a9ca481a83299","0xbee5e61838f19180bf3b2d6a1f5457be9449060a77cf997f9899c2c0abbe8bcb","0xb8772fe626c2a3151e5c2947b51f80519df6cdff0ba34d9b22a148d9e88740b1","0x89a3606507a02b2b879ded73d8a21240dd53f5e8b3a2733458d773de8368022c","0xd87c56af4fdc483cb66935d7b47afcc0f545098a0c689f6baa102ed52debdef2","0x7b49ea42028f098d46b330bd4b15422004cfdab6194de4307d6465fe3842dffb"],"transactionsRoot":"0xd6a76734c1194ba7a54efdf597b6bbd71a7c258df83e17ae24c61ef55ed6f5a6","uncles":[],"withdrawals":[{"index":"0x4fd5dc","validatorIndex":"0x343b9","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e3737"},{"index":"0x4fd5dd","validatorIndex":"0x343ba","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e6b61"},{"index":"0x4fd5de","validatorIndex":"0x343bb","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e609a"},{"index":"0x4fd5df","validatorIndex":"0x343bc","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e446a"},{"index":"0x4fd5e0","validatorIndex":"0x343bd","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1ec58a"},{"index":"0x4fd5e1","validatorIndex":"0x343be","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1f08f0"},{"index":"0x4fd5e2","validatorIndex":"0x343bf","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e388e"},{"index":"0x4fd5e3","validatorIndex":"0x343c0","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1de6e9"},{"index":"0x4fd5e4","validatorIndex":"0x343c1","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d8671"},{"index":"0x4fd5e5","validatorIndex":"0x343c2","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d921d"},{"index":"0x4fd5e6","validatorIndex":"0x343c3","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e47be"},{"index":"0x4fd5e7","validatorIndex":"0x343c4","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e68a0"},{"index":"0x4fd5e8","validatorIndex":"0x343c5","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d4535"},{"index":"0x4fd5e9","validatorIndex":"0x343c6","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e9eee"},{"index":"0x4fd5ea","validatorIndex":"0x343c7","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1eb96d"},{"index":"0x4fd5eb","validatorIndex":"0x343c8","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1da634"}],"withdrawalsRoot":"0x5f2d20dfa1ed20da2588eea50327afc1b477e2ad628b8d40c5eaddabbf7d32be"}`,
			},
			xc.TxInfo{
				BlockHash:       "0x03d633a561d2217f8d7ae529ed90f3f4709fd62a5fb1b0ff6f7ce487f2113ba7",
				TxID:            "7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b",
				ExplorerURL:     "/tx/0x7fba5ad368aab2490731a31d490e22d905c9b47ac2ca03e41b2021bfb76b423b",
				From:            "0xE8Be958f910FB1bb439EaFBcFD0475509AB6D43F",
				To:              "0x5D2EBDf613D50Dc598A09d8Ebdc3F285bE6CF8ed",
				BlockIndex:      8983848,
				BlockTime:       1683845580,
				Confirmations:   19,
				ContractAddress: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
				Sources: []*xc.TxInfoEndpoint{
					{
						Address:         "0xE8Be958f910FB1bb439EaFBcFD0475509AB6D43F",
						Amount:          xc.NewAmountBlockchainFromStr("10000000000000"),
						ContractAddress: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
						NativeAsset:     "ETH",
					},
				},
				Destinations: []*xc.TxInfoEndpoint{
					{
						Address:         "0x5D2EBDf613D50Dc598A09d8Ebdc3F285bE6CF8ed",
						Amount:          xc.NewAmountBlockchainFromStr("10000000000000"),
						ContractAddress: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
						NativeAsset:     "ETH",
					},
				},
				Fee:    xc.NewAmountBlockchainFromStr("51970500381117"),
				Amount: xc.NewAmountBlockchainFromStr("10000000000000"),
			},
			"",
		},
		// Parse multi erc20 transfer
		{
			"multi_erc20_deposit",
			"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
			[]string{
				// eth_getTransactionByHash
				`{"blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","blockNumber":"0x89152f","from":"0x489a3aa83c1f204f0647c67c9fbf3e7ee1463bc5","gas":"0x61a80","gasPrice":"0x59682f0c","maxPriorityFeePerGas":"0x59682f00","maxFeePerGas":"0x59682f12","hash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","input":"0x85ff842c00000000000000000000000000000000000000000000000000000000000000c0000000000000000000000000000080383847bd75f91c168269aa74004877592f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006100000000000000000000000000000000000000000000000000000000000557300000000000000000000000000000000000000000000000000000000000000014489a3aa83c1f204f0647c67c9fbf3e7ee1463bc5000000000000000000000000","nonce":"0x71","to":"0x805fe47d1fe7d86496753bb4b36206953c1ae660","transactionIndex":"0x15","value":"0x6a94d74f430000","type":"0x2","accessList":[],"chainId":"0x5","v":"0x0","r":"0xeddde218c0079cffd7e496bbbcbe31689754af4ce18bb1407d3d9dffee7a87ea","s":"0x48ed7ad3f32c037af91d575c054e48a78d78c21bee76a4148ad5e2f39475ef65"}`,
				// eth_getTransactionReceipt
				`{"blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","blockNumber":"0x89152f","contractAddress":null,"cumulativeGasUsed":"0x28e666","effectiveGasPrice":"0x59682f0c","from":"0x489a3aa83c1f204f0647c67c9fbf3e7ee1463bc5","gasUsed":"0x2862a","logs":[{"address":"0xb4fbf271143f4fbf7b91a5ded31805e42b2208d6","topics":["0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c","0x0000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d"],"data":"0x000000000000000000000000000000000000000000000000006a94d74f430000","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x1d","removed":false},{"address":"0xb4fbf271143f4fbf7b91a5ded31805e42b2208d6","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x0000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d","0x000000000000000000000000b3a16c2b68bbb0111ebd27871a5934b949837d95"],"data":"0x000000000000000000000000000000000000000000000000006a94d74f430000","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x1e","removed":false},{"address":"0xcc7bb2d219a0fc08033e130629c2b854b7ba9195","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000b3a16c2b68bbb0111ebd27871a5934b949837d95","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660"],"data":"0x0000000000000000000000000000000000000000000000002f3935c5c5e2d7ad","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x1f","removed":false},{"address":"0xb3a16c2b68bbb0111ebd27871a5934b949837d95","topics":["0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1"],"data":"0x00000000000000000000000000000000000000000000018791b4f45022da96ed00000000000000000000000000000000000000000000ae03fa18d28e355548ce","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x20","removed":false},{"address":"0xb3a16c2b68bbb0111ebd27871a5934b949837d95","topics":["0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822","0x0000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660"],"data":"0x000000000000000000000000000000000000000000000000006a94d74f430000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002f3935c5c5e2d7ad","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x21","removed":false},{"address":"0xcc7bb2d219a0fc08033e130629c2b854b7ba9195","topics":["0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660","0x00000000000000000000000000007d0ba516a2ba02d77907d3a1348c1187ae62"],"data":"0x0000000000000000000000000000000000000000000000002f3935c5c5e2d7ad","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x22","removed":false},{"address":"0xcc7bb2d219a0fc08033e130629c2b854b7ba9195","topics":["0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660","0x00000000000000000000000000007d0ba516a2ba02d77907d3a1348c1187ae62"],"data":"0x0000000000000000000000000000000000000000000000000000000000000000","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x23","removed":false},{"address":"0xcc7bb2d219a0fc08033e130629c2b854b7ba9195","topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660","0x00000000000000000000000000007d0ba516a2ba02d77907d3a1348c1187ae62"],"data":"0x0000000000000000000000000000000000000000000000002f3935c5c5e2d7ad","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x24","removed":false},{"address":"0x00007d0ba516a2ba02d77907d3a1348c1187ae62","topics":["0x7ec1c94701e09b1652f3e1d307e60c4b9ebf99aff8c2079fd1d8c585e031c4e4","0x000000000000000000000000805fe47d1fe7d86496753bb4b36206953c1ae660","0x0000000000000000000000000000000000000000000000000000000000000061"],"data":"0x000000000000000000000000489a3aa83c1f204f0647c67c9fbf3e7ee1463bc500000000000000000000000000000000000000000000000000000000000000c00000000000000000000000000000000000000000000000002f3935c5c5e2d7ad0000000000000000000000000000000000000000000000000000000000055730000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002800000000000000000000000000000000000000000000000000000000000000014a0b5cbdc4d14c4f4d36483ec0de310919f3b2d90000000000000000000000000000000000000000000000000000000000000000000000000000000000000016065f49bd49de252a7f0d9100776c70f0da398368ef9866f8e21fbb0e3e630e74f000000000000000000000000489a3aa83c1f204f0647c67c9fbf3e7ee1463bc5000000000000000000000000b4fbf271143f4fbf7b91a5ded31805e42b2208d6000000000000000000000000000000000000000000000000006a94d74f4300000000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000080383847bd75f91c168269aa74004877592f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000014489a3aa83c1f204f0647c67c9fbf3e7ee1463bc5000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000","blockNumber":"0x89152f","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logIndex":"0x25","removed":false}],"logsBloom":"0x00200000000000000000000080000000000800002000000000010000080000000000000000000200000000000000000000000000000000000000020400200040000000000430000008000008000000200000000000001000000000008080000000000000000000000000000000000000000000000000000000000010000000000000000000000000004000000000000200000001000000080000004010000000020000000000000000000000000000000000000000040000000000000020000000204002000000000000000000000004000000000000001000010000000020000010100000000200000000000000000000001000000000400000000000000001","status":"0x1","to":"0x805fe47d1fe7d86496753bb4b36206953c1ae660","transactionHash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","transactionIndex":"0x15","type":"0x2"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xc","difficulty":"0x0","extraData":"0xd883010b06846765746888676f312e32302e33856c696e7578","gasLimit":"0x1c9c380","gasUsed":"0x40ffac","hash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","logsBloom":"0x802508400400a6882000001e8003a0849018081030000c74010300130800100004a0042052000600104002081144000001011014c008002001800e0c40ac244010064432447000001a08d08a000013280084001000ca38488008902080a80008040c04004a220040814000a40e0008008000c8024048415088215010200010001110a0e01004a00a28400840800000820200860110104288000021401406800876880006780003414000204228000100240002012084220423000194a4e00140102d562304200000840220412540401c21081000008006103081228000022108805010e425680304a08040a02880d000040092000c0022c400024008b0804441","miner":"0xf29ff96aaea6c9a1fba851f74737f3c069d4f1a9","mixHash":"0xdf968dc511b6d20aaa6b6fb1f184fb12721c98dd1b60e7fd0ad05bf6bafaae45","nonce":"0x0000000000000000","number":"0x89152f","parentHash":"0x1d62529b39fb2a9128f0a6974d6884cc83d597b2b28ecceb2481592aeab67c7e","receiptsRoot":"0x91de28df8bdcba2a225a6055a91eb98a3d5077851f72552fd2d01ee300204785","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0xbcea","stateRoot":"0xf341dbc3ffa26b025175b0b3ae12486414dd708968a873533f431305d0593ed0","timestamp":"0x645d7238","totalDifficulty":"0xa4a470","transactions":["0x9ddd0ba4a7b6d1e0208f89c3bb8f8fb3c08f62cfb7455d9c653e83e852e1be10","0xc1fbc8d97a9555595aaa2b042349fc80aa2c81a3d25e8c67b99f251ed9c57a8d","0x44ad22b2348477b4e5d4ad7be40b1c9ca9d33c48479f00138e04e1e344b7d8a3","0xe39bb91cd27c4c2cfd4355bb9f1fad043ba90ce9d36e493ad9dbff7f86e7ff3f","0x8baf664193be9f32f4309dfe67843f69ff504466aaa3e0d3b0a88d1d3b0cd566","0xcc1ef220654fa3f8d553c9dead5d3586f5903ccb2dcd5c63a8a0651ff651a706","0xcc2140f71456993f0c693d343c606b0201c7a527d517349cdb398890a7ce881e","0x26765934629a0f4c6eb217ce33277be54c7e89dee19733421eaadc56b49a6881","0xfd4f08359a57750c832fb8bbb3aeb5f65c1a9155114385b5646105ad43825a67","0xb392bfbe5797b65b8e53adef85cb106d674b781b36288e436b6914384a64a42c","0xf0bdde2b3ac7594ab1fe87958322ecde59b51c128d9ab4c21054a3c7311c2140","0x51e8066ef4adc92f82975a83c23f9d7f9ed59fb614767b232267d5a50ae23709","0x7e60de38358ac757a873c07dacb7e82212b70686e8935933b03382bbfecaa8c3","0x4fb4226d7755c74bd9d3e2a3a3b46204c8b2b2ba825875e2f9f57590121d758d","0xe8a60c17e6211ac5529d1c0957da7ace732e78be0baccaa33afb3312c15f25fe","0x71b74a28b9a52dc76461c7c32c0bbf505920d86700cfaa1be93debf02e25275a","0x484577c602a92633b84f0de8c3081136c0c83ba45f72a6c7db13cb5c001812ab","0x59b0b540a35ed6af08f7413da0c1e25f054c51feff830f44b25699e3e1f6ee38","0x5758a506e2701e95db58ad7e1465afd6a25358fc1d9acac1f93a060de5af7fa1","0x10db0e3f366adaa72d1c43a506aaa17ef2e32cd390badd6258f52798093d1c3b","0x69e4050d3f0f6d2c71f11d2d065705eec9364b7330d4ba7acceef35a7d10c6c8","0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","0xf18ac7d92debbd65ef45f676d427dc3699ab01f2a1d7eaf17f307a39dc9e7538","0xff1f43c3f9b8a3a7666ac23407628324169f7049e7a0043c37eea060d2048f58","0x44b34923c696bbf4bb8574d3581f3ae0de3728d8780a776929a7fec1373ef392","0x8712e80897ea4072ccc629ebf237868a911ac82056eb773030e74d3775c95a65","0x956fe598a2d87a7106c9aba97690b833bd39bca4d6468232fd277875524447e9","0x8b8ef48393e6cb80ae1ca0e09db9c3737b14a3f7a2cd40b21426f3b6448b5b80","0x7ed440bbcbacaa90f0bf6ca79aaa9d5e59f11f4a0c362d5f18537cf9ffc64074","0xafc4c18df696a09354aba0a23dad2204cf315d5603b73f2006068939a8428353","0xa1fc74af0bb980bcf94177992bbcb233aa8a56c133033e5203498098d69ed6f4","0xb881c161c4958fa5b0a07ed2028f0cf3f2e6647f2a8e85c31c8a4b3da4dc4762"],"transactionsRoot":"0x73b351a7e242b5dac0ae463cc9a6c6d4330bce6210ac9eed84c0149d4d5a0117","uncles":[],"withdrawals":[{"index":"0x4fd51c","validatorIndex":"0x339ec","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1ad1e2"},{"index":"0x4fd51d","validatorIndex":"0x339ed","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1e2de7"},{"index":"0x4fd51e","validatorIndex":"0x339ee","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1cff10"},{"index":"0x4fd51f","validatorIndex":"0x339ef","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x198edc"},{"index":"0x4fd520","validatorIndex":"0x339f0","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1ba6e9"},{"index":"0x4fd521","validatorIndex":"0x339f1","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c2ad7"},{"index":"0x4fd522","validatorIndex":"0x339f2","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1afacd"},{"index":"0x4fd523","validatorIndex":"0x339f3","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c9b7e"},{"index":"0x4fd524","validatorIndex":"0x339f4","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1ca79d"},{"index":"0x4fd525","validatorIndex":"0x339f5","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1b604c"},{"index":"0x4fd526","validatorIndex":"0x339f6","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1d772b"},{"index":"0x4fd527","validatorIndex":"0x339f7","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x19a5ef"},{"index":"0x4fd528","validatorIndex":"0x339f8","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1cdc1f"},{"index":"0x4fd529","validatorIndex":"0x339f9","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1d7e64"},{"index":"0x4fd52a","validatorIndex":"0x339fa","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x1c654d"},{"index":"0x4fd52b","validatorIndex":"0x339fb","address":"0x85bc9b9da65694345f2a360e46c7abeed8df895b","amount":"0x19dda1"}],"withdrawalsRoot":"0x43220e00b40d79d4bf5efc17d53128232254ed9e0be21df6947123feb5b2c733"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0xb","difficulty":"0x0","extraData":"0x4e65746865726d696e64","gasLimit":"0x1c9c380","gasUsed":"0x411183","hash":"0xfab7fb7e5d36366c47ae086992949daf4f1ff56488b0da8003591c162e54ccb0","logsBloom":"0x8000120080284004000020030195024400030100004110080008000008000000034880010080820000020108000000400000100400000020020404100160800020004000988000000000000a000810060225004000800080004841004008000001002104020480400000000000500f8100014000002043810000401c00000008002a000008001000a0000924020a041000050010000002000000000100400410020400041100040800001000144082100441428680c801424002000601020020809000020100000085026000000000000000d00040000080100900110100240208900801246000102034020d02c4003082010110001902848002000001010440","miner":"0x000095e79eac4d76aab57cb2c1f091d553b36ca0","mixHash":"0x79788fc0b75dcc9523901ae5aa44a6bce981cc88a03006998fdd686e0c497f32","nonce":"0x0000000000000000","number":"0x89155e","parentHash":"0xb54b4ee55f85cbad2545bcd2e65949d8c9141849a3c8e63a3ecfdf47a15ac11a","receiptsRoot":"0xba2c9155ca1d0c30b8df13f7edb42da9f5d0982eeadf449c68eff01bdaf608b0","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0xbb5b","stateRoot":"0x7d3e4185f85f2dedc56263238cf0c6b1f20c190a913f7a0d313017c605c381ac","timestamp":"0x645d74f0","totalDifficulty":"0xa4a470","transactions":["0xd1f8c9dc13e95674313b1a68d6886c4af50b0db585e51fbccfc542039b2906c1","0x6eef6929d592942fc33f320fc1eeab297374823ee4288e85647c4a7272dfcd30","0x062899dbee1eadb3e04750e3d634605da9f3810a36c9506b96f8581fd46782f3","0x7d60969bee5d67c811d5e3be122fee29b29502691f26dfdf2ee0ad066cebfc16","0x5fa5fe97b5df6d1b4228b5440d72d19591e6a6fef0cc4f5149f2662a777fc072","0x326c0541499679643dfe5d1e68418e8f1ac1638141edbd1ea9aa81466893b064","0xc22781323752fbf5db6650b5111f3d32e5d935a9fd395ed677191f1ded0996a2","0x23daed3af731f2c1783956fbfefef441541d39e9fcc3f5fd03f769423f8e07dd","0x8c549220b3a0c1de2be998412ff96562e5a4d0e7c4916d59f2d58cdfbbe15c9a","0xaf72951e96959ebd188e012eb4097180c0c3aef0a4e01ccff895a02bbc9f467d","0xc49ff950234d88cba3f26ff19ea19a73e4d3dc5d29d2f194249f1b1dfff51a64","0x4d6a46d288144b179f34e812d32770dd7771f7aa9981d8eb710eff806d82cf07","0x4e52ff96e4a1b3a402b4cc204bc744a6bf0cfb899f9ec9d954d9c9a6b299ba85","0xeda7afa61cbae6af5d59a37939fdb3c45c19e90c5dbb1763c0615a4434ea12a1","0x7536b273e1d95f89776a7e554bfeb7e31829abe1b744ae9500945810d5786d4b","0xb3300adbb2dff7f02e5d95e9f6d2d382d8751a4d508eff96c5cae876e0a7c525","0x6c69ada417dd8f85ce552a65284a8df0f01ca47c1f4f11e23ea160e4b59177a4","0x8a209655a148cedd506fd293820f7e81f044a77e54c534d4cc5a8f47c294b6d5","0xdc4ba5332200de1bde9be4077ddd14534748cc5f08099198c4ec265b9902f505","0xe0929869936568afd6a855af710392f2b31b5c6ceddbbb2d27efb53cac13dc55","0x6a3aa8a16fd622ca47d3ff684737acbda22d5d984c9d0a60d533b7c077f5b4d6","0x9623aded68f1a26022e873d0407e5cd436e6e21540733794db1ae8a906e73946","0xd479b0dd8f09e13cb1c05cf7f558a025ef7cf0a4b9af6e59c092b5288ee56b53","0xab9da6e93e805971b7ed4048e1931f2a9a83711792f62c3cdcff9acf98ffbbbb","0x658042508ec734452144a452c049403df152e84267ee5119fcf5d32e1f552c45","0x774e2df2c49b185bace3af75cb788ec162ebc4976f319d54b19832428045748d"],"transactionsRoot":"0xe8cd884359b2604683ff552a25dcd20a6d44a8eb11f5a23f32760160fafe0cd3","uncles":[],"withdrawals":[{"index":"0x4fd80c","validatorIndex":"0x345e9","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1ec58e"},{"index":"0x4fd80d","validatorIndex":"0x345ea","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1f03e2"},{"index":"0x4fd80e","validatorIndex":"0x345eb","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d9006"},{"index":"0x4fd80f","validatorIndex":"0x345ec","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1edc15"},{"index":"0x4fd810","validatorIndex":"0x345ed","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1ec0d3"},{"index":"0x4fd811","validatorIndex":"0x345ee","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1dfa69"},{"index":"0x4fd812","validatorIndex":"0x345ef","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1f1c2d"},{"index":"0x4fd813","validatorIndex":"0x345f0","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1dece2"},{"index":"0x4fd814","validatorIndex":"0x345f1","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d6d25"},{"index":"0x4fd815","validatorIndex":"0x345f2","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1eb680"},{"index":"0x4fd816","validatorIndex":"0x345f3","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1f2c57"},{"index":"0x4fd817","validatorIndex":"0x345f4","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1d92b0"},{"index":"0x4fd818","validatorIndex":"0x345f5","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e1b8e"},{"index":"0x4fd819","validatorIndex":"0x345f6","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e480d"},{"index":"0x4fd81a","validatorIndex":"0x345f7","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1f37d6"},{"index":"0x4fd81b","validatorIndex":"0x345f8","address":"0x9427a30991170f917d7b83def6e44d26577871ed","amount":"0x1e3d26"}],"withdrawalsRoot":"0xd35e7c907fbcafc91dfef93f1db5e612de63a16e3d60484e61c708420415bff2"}`,
			},
			xc.TxInfo{
				BlockHash:   "0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d",
				TxID:        "b3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
				ExplorerURL: "/tx/0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
				From:        "0x489A3AA83C1F204f0647C67C9FBF3e7Ee1463Bc5",
				// to is now the contract address of contract making multiple transfers
				To:              "0x805fE47D1FE7d86496753bB4B36206953c1ae660",
				BlockIndex:      8983855,
				BlockTime:       1683845688,
				Confirmations:   47,
				ContractAddress: "0x805fE47D1FE7d86496753bB4B36206953c1ae660",
				Sources: []*xc.TxInfoEndpoint{
					{
						Address:         "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
						Amount:          xc.NewAmountBlockchainFromStr("30000000000000000"),
						ContractAddress: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
						NativeAsset:     "ETH",
					},
					{
						Address:         "0xb3A16C2B68BBB0111EbD27871a5934b949837D95",
						Amount:          xc.NewAmountBlockchainFromStr("3402810116999927725"),
						ContractAddress: "0xCc7bb2D219A0FC08033E130629C2B854b7bA9195",
						NativeAsset:     "ETH",
					},
					{
						Address:         "0x805fE47D1FE7d86496753bB4B36206953c1ae660",
						Amount:          xc.NewAmountBlockchainFromStr("3402810116999927725"),
						ContractAddress: "0xCc7bb2D219A0FC08033E130629C2B854b7bA9195",
						NativeAsset:     "ETH",
					},
				},
				Destinations: []*xc.TxInfoEndpoint{
					{
						Address:         "0xb3A16C2B68BBB0111EbD27871a5934b949837D95",
						Amount:          xc.NewAmountBlockchainFromStr("30000000000000000"),
						ContractAddress: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6",
						NativeAsset:     "ETH",
					},
					{
						Address:         "0x805fE47D1FE7d86496753bB4B36206953c1ae660",
						Amount:          xc.NewAmountBlockchainFromStr("3402810116999927725"),
						ContractAddress: "0xCc7bb2D219A0FC08033E130629C2B854b7bA9195",
						NativeAsset:     "ETH",
					},
					{
						Address:         "0x00007d0BA516a2bA02D77907d3a1348C1187Ae62",
						Amount:          xc.NewAmountBlockchainFromStr("3402810116999927725"),
						ContractAddress: "0xCc7bb2D219A0FC08033E130629C2B854b7bA9195",
						NativeAsset:     "ETH",
					},
				},
				Fee: xc.NewAmountBlockchainFromStr("248127001985016"),
				// amount is the first destination
				Amount: xc.NewAmountBlockchainFromStr("30000000000000000"),
			},
			"",
		},
		// Parse multi eth transfer
		{
			"multi_erc20_deposit",
			"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
			[]string{
				// eth_getTransactionByHash
				`{"blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","blockNumber":"0x1087311","from":"0x7830c87c02e56aff27fa8ab1241711331fa86f43","gas":"0x1e8480","gasPrice":"0x9aa2354f8","maxFeePerGas":"0x1229298c00","maxPriorityFeePerGas":"0x3b9aca00","hash":"0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","input":"0x1a1da07500000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000015694000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000c1f0e841572ccfc0b85a84f4ab532571701085d5000000000000000000000000000000000000000000000000005691830a7fb00000000000000000000000000095bc53640b38ab48132cbfd3b058fe625b40f55f0000000000000000000000000000000000000000000000000059476486232400000000000000000000000000fdb0171a63795629b27ca715ffd09c85e82c37be00000000000000000000000000000000000000000000000000632ec3863d0000000000000000000000000000d20fbde842ddbe5110a64f6e7b45d33fca0d78b200000000000000000000000000000000000000000000000000abfc66ec5e7800000000000000000000000000085a07ff3fa9635de9771dc60f8c37d2f12c05f000000000000000000000000000000000000000000000000000b86e2275db24000000000000000000000000004368e1b4ce65eac80d1c1f063071265e5d85146600000000000000000000000000000000000000000000000001250f7fdbda4400000000000000000000000000352232369aa3b2c551a47fe16ed1052bb624dfd30000000000000000000000000000000000000000000000000161585c3f3c180000000000000000000000000055786c2009e833c198f43f7793ae6d9c7d14d31c000000000000000000000000000000000000000000000000016345785d8a000000000000000000000000000089d079bbaaf3fc0ceafb94df622104892027c332000000000000000000000000000000000000000000000000016ef0a8f91254000000000000000000000000006112456a34ff9b583985ab3cc7125bbc2b7b17f600000000000000000000000000000000000000000000000001f008506eee4400000000000000000000000000a1cd502f9c91b4fcd234d83a5d63585a2fdbadbf000000000000000000000000000000000000000000000000024945c8c664bc0000000000000000000000000000007d0BA516a2bA02D77907d3a1348C1187Ae62000000000000000000000000000000000000000000000000404110781e0f0000","nonce":"0xba6a6","to":"0xa9d1e08c7793af67e9d92fe308d5697fb81d3e43","transactionIndex":"0x56","value":"0x0","type":"0x2","accessList":[],"chainId":"0x1","v":"0x0","r":"0x86386274eefbb41bcd34d8f2cbfdb2b7d14cab4fc54328df4ca12285b036ce2a","s":"0x4e5f21c0352e2e27bce327e88b570350092c1e21401f503a272410923efd9a59"}`,
				// eth_getTransactionReceipt
				`{"blockHash":"0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d","blockNumber":"0x1087311","contractAddress":null,"cumulativeGasUsed":"0x622475","effectiveGasPrice":"0x9aa2354f8","from":"0x7830c87c02e56aff27fa8ab1241711331fa86f43","gasUsed":"0x24a76","logs":[],"logsBloom":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","status":"0x1","to":"0xa9d1e08c7793af67e9d92fe308d5697fb81d3e43","transactionHash":"0x1e3db73c736e4e2757f348d196cc1c86bf47fd979af72252027242eed22325b4","transactionIndex":"0x56","type":"0x2"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0x96e888af8","difficulty":"0x0","extraData":"0x6265617665726275696c642e6f7267","gasLimit":"0x1c9c380","gasUsed":"0x1bbcc44","hash":"0x50f45940c6faca69d4eb42348df92cdebdd3eef279c09b37318a52af0b8dfda1","logsBloom":"0xc5f76b1969973577d31b8b7cc77b1fa5b8e5300f4856901f90239008d5b8bf591fc21bd0ea3a573450b05841faf0e14d9a397f4d9e7eabe322d0c1de517fee044fceacaf5c7c2978696be7dfc524b1afc4ca976d994e1feabe9a5c6ed9f7137bd2c66ee33f228735ecbeb320a2f8fedfa07371be4af14f23a8e016be797b8644869777dc72986c7996efdde2eb4a51a4abfbf5adfbcd9efd5ca704437a197dac9ea8a162f7386257c8f263f01c15ac080dfb874b70396fb636a7267640b8036b17382c1b24055d73b908f381a6ee9bd55c684aab4648e8dd5fdffd7ffb57e84e1e32b1bebb25b1df6e46dfa6ce013be493233afd755e00daba9d9f31cc12d499","miner":"0x4675c7e5baafbffbca748158becba61ef3b0a263","mixHash":"0x0943a3f23e0fe2fcce2100810f555bb6d54dc58546f8c7fe6d9214c15df71452","nonce":"0x0000000000000000","number":"0x1087311","parentHash":"0xac095c3103b356735dc33a3041067a4dafdaff8527223e4a926c7dd8eab3d5d9","receiptsRoot":"0x893f72e987a78d66e856299477b3acd760023675bde0cd958f13ce999d90f095","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0x15477","stateRoot":"0x9874175fe32d20f079526cd721fb2a730fe3ca09cde664a57ac4a02cff3ad089","timestamp":"0x646e5d8f","totalDifficulty":"0xc70d815d562d3cfa955","transactions":["0xe7b3968958e1aae9ad66e0c46a1b546f9ee45a97e57f4271b94e1c5a571655d4","0x51c511271a2cc7736cd40b67f41ff3cb1ce7ef41b4ac42b12d10936f1a3efd15","0x2bd6e966f53d175a1453f0893eea401345014b06d3c8357fd1de0b4e4c190821","0x76d019b0986a76a937804b3014c188fe4f20cdae842acf6d11a2d6ec0ab50098","0xd7a9c5b1d81c4507d6914c438a8a58c03fc4f7ad51caaab5d15baf23382171ee","0x5395bce1ffb99f2bee37be8feef0a93209b6f63bf84087e05237547db0b2ea7c","0x12efa69a3d52d1c156954cf78ab72a598550e3b8157d167af72e1251015b6f14","0x0d7d1dc12653cfe5d39f37bdbe8b7ebd6d9552d1932e3f5e9c96fa11d9c1e571","0x250e5d9d05ab5c44f2ab09dd3fe756384487d03ee105892bb50da1b7c4d5fd8b","0xe640837063a09a0e11784b561a1a5d96c5dab748034509312a60debed215ef0f","0x24bb0be3d488b9f387b935afb78f3598ecad87fd6f1aac2777ac114b110ee144","0x0424d5989187b2a51ca1f3e36374e3204485d12e36508532329e0c8d2cc0b26e","0x3ff8ffda2fe013d700a841c5678519da4a2b29df81f710e0cc508367f5a46042","0x82b31338387eaed404c8270ceaddf4768d21649abfb5fb16e3e8ec87c1b16611","0x8beb916eed8e8ac926770413575cf64f88d71bad63d05bfa66240d8e7875eb08","0x9dd4352951fb80afdd8741a3870aa94a5a0a52f634c1b76aea881363b7238340","0x30927c92d4a975acd070b284251aab8cc6b8d0f3b703278668b7d03fe63819cf","0x92221f2d89d4c59c1708bb76647bbff94ee5987232cd593161ad32a7de1377da","0x3e68498851d7350cfa61ea31f5275f11da9efd58fb2f50414cc39a9ab6651546","0xc7213027d31375114485d74e037c8f6cfb6dd0735fefe25fa366d5c03287132f","0xfc0075c491eb190ebdbe753cc270c551340d976a21a543aeb44d651b41fe686e","0xfe03ae4d2d974a48593b272a0799463c16202f4d32ac3b933b1473a7df850b8f","0xb8e5d1ae245bdaff89ba8449d8a4791b656ed6289fcab6efb363da07d89f2906","0x083837f3b20b1cb93e349ecdda70664616c8d300c41d7e6a517f1a169c5e8bb0","0x4d5ce7266422da6815281e3fceb3c0d58519088b763b257948608841391f4638","0xbb847f33c4a46891c22e7c457e6e017640c47c1619fc70d79932d69debbc98a8","0xe2c6b019e234ceb299f9872cbb4646960b73213536265a7c3b782a32120155c2","0xa6e9467ec86a101e95d1bae50df24a695c583acc09d654790c3b99d2b67e5a5e","0xa1bc55c3b78dd2cc4d299cbf261f0e999309007b99182ffd7f2bf5f5bcfaf99e","0x82d8676c40251efaad2839b12d75065dc47989dc96df18c24ed190727333e18a","0x049e11c1cf730c872d5e63f5335c1ad07c93d7dd7c641184ba543d8da3171590","0xda3bb471957b435fee432733f004ae706bec8715c23a0143a33b14916336d7bb","0x09f363fef8d223742a836371de091185db55e91870dbd458c31d3772a8700701","0x9ebe57749606c5b95cc0b1a6675857b8a997c62f5fbb30d47f06b2a23932aea1","0xdfe38c1ae8d551f54ca4a4cd65cfbdd5e55f2262c002170e2e09b07623b29b33","0x01759daaa3790f8e567c1c7668104891e1b9a71d0bd87be52e7a07e988162353","0xa66cc968efa45a735d3b833be29cf96c9f192fb432b64e920433b6ff020acdea","0xc50f39ece619f79d5c4671f40cef93d50e4f97e82d78a07ec915f66aeec6c574","0x9c506365d48eac3a258fff600c44af619a86d0937f970ef52863d6507bc2d773","0x4d29862c9d53d648f7b5b59e338ea002c73806a7d0c99250c685b595cba8b037","0x72cb55183efa00ea5ad560722b0c948d600d47f440bb4f12fdf6214fb67ebbd3","0x6970409a4adf2da952e381ef1f1346cbc0ff6cb7e0f9dc093eac4f86162ec311","0x542187bf5278ce7ebfb8eacbb6a528b47563993f38ec19ffeb6b1046858bcedd","0x52ec73a474bc0138009232b64f97de6c06886e5da409ec714c2d547b0e816f77","0x34d5275c60d7f9d23b6f6f6f7d7bad15ca076a67f2e135b9be7a750b4f352f2f","0xca1c8b4b711527b329a7b9ce90842ed6acfd8919baddb61f4715acd332cc2b26","0x06d1bf68723888ee638ad83302d93afd101cf0ac379215717748af79bd102733","0x877ddf1e63288939ebb7dc519d65ac667970ed2225a8db9dcdf75ba25596b9a5","0x4d98d67a9360ac96410b7dc53c310f5717a5175fa0779dc239ddff5089f61bbd","0x287298d55b3812d0a864babe5379d9b6e9a1d36d715a75ae0d61bfa79dab3e7e","0x533feb6df61bbf6e3cc9eb5619c8726e8cda6edf080ffa418038530615c3569d","0x21fe2f02b229bb97ae38833b47d52dba172f9adc6587ae8d5225c1d7253d648a","0x98e8acd4c019b61b48e9e39ac62ab7b95be568fd65c74a137f2ddb4e06e75d39","0xb7fd0edfa062a6e6681f78d0feae228058b88c5c1d5d5e95aea863355630abd1","0x98b07b710a61a408a339d33a4a22fbb05b586bbce87d628d2d08b4e38d87ab68","0xd86fa095a687ee468650b2a04ed3cace32f9c17ee58711acb20fbf3cfdf2cfea","0x50a9694ff23ce4f7db87bff239b1aa9910b3efec346e17696604a713c858c6bb","0xdce7a9aa7a2740cd96ce9a88970398ffef58a9b8741599415c8b74f64c35dc40","0x5ef284de6578f00b378b0ae1d977b99fa08f25e9550d99418feff9b581c2daea","0x8a8fdc5f101b14d2bc9dfd01b2ab9c37f48f637480f5a41ae1f66f265ab467cd","0x006f3bf082217869393b644ec0f4681ffab639031f5d5a0d86153ef1c28ac81f","0xb92bf871125498e150c41495a005023bc730403e3c3cd01867f2a0ac878baf51","0x775c0df4c90e45828022347d402e8745fd9357a017ff022b44ce693c43d38b29","0x655f770a5eef15ceb56da758ecb604ac0fc40151686f48644ef87dccf0e25d92","0x03f8fcddafd4afcc7ba2bfe0ed88a569a03b0d4afbd291a7052c68f8dce5df38","0x8243aecd8583df302a96dbd752db9282c8395b34b4352c0fa80f5e0830e9052b","0x46bcf82b18cbaec848ff8e88a87c03e3ee6442ea29476eb8237fd13c4b78714a","0xa101e6f5a221128e21d29043b25dc7edba929f5c8cdb4a4ee30a7795ec674564","0x86b932e1c7557a0b2817aa8208f0463d7c73fcb3425559493129018da0e90745","0xae13401cf3d8cb0f09ea6b75d6c84b967314ca77eef8e005dca5f51d92194d24","0x6cae8372e8ec85534f3e1205f39d28940d726d27c49cd5d256c912c31686bc91","0x6da9cf233c605043f720b2acec5bed87e8f236c746e2199dceb79dd90874f7c5","0x047e3c8ee8bf3f92fc07d47b59e8c0bc49b16c223aba72c6089f685d6aa90e7a","0xdd774e44120cf7efedc8a7d3d977db3c812f3ec50dc69a0de59192bb543c56ed","0x8c3d695f0edc3b9a070fe275c9f28e9fe6e641f3d1a74c1c6f0c0578b4692020","0xd49a7d5e31fbec2e63187ad5c6bfe256c6c5e4ee4b37c75c9343bf818d70af1c","0xfd85a766e7ef3fac66af3541313bfd124f08d426eee7403a82ec980cea988893","0x461fbf11d756c538da64478ad959cc80866298a0baea48b69d7a335b43f2135c","0x5f84c754b4172e549cba79b81c70bac631fbfdc96b4cb53a11fdb6c94e8587a6","0x63ed84b6e9221396e35e42ea1345845f6a131f4b504cfee915192bee30f3af76","0xddb7f0654547a5272af1114c82a029f3e5fde4bdef7a36c6e8b1bb37d3de629b","0xe865b01a846c3440eb4f092a98b06bac43e7f06b68a3f3e98d3d235c25d78886","0x8e5864b24936db993900204164e90a4b3cdde6550870b74af89ae4963caedb0e","0x6bfa82237bf299f1f054d5e14653e0154fc68b2dc9e4845b0b621f9ac1a6ff91","0x08a52ca96500b59794c420b6137504e05161ecbd2aca2f66149cc157e6c07bd6","0x24a8d9d908af5a2dcf76684b24eb910ff10655da354eece344ba555369152580","0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f","0x762e17f88252469eb771a043c620b95ceec8a0aa8498d9861112eb790005702f","0x007561bb5764fb0a53133cfb5ba3b60108f33764437ebdb338aabf50954afc0d","0xc2b1f3ed1de14a96ea5021a09e4fc7a5a2e9c7c4f05880a629269a65f9661cea","0x109fa5a95209f22d0862b7e37a3b26d08ba880ef86b12521c56f2e17409734df","0x18e49058cdd6924acdaf563be060b0428344041c3d4c2d3750b9f729f34394d8","0x134d6a7496f9a2bfc7aa9f7acc91c7c095cf6bc8f2fc4d2fc9b35aebea7880b3","0x3cfee56311ef5f4f9aebb9ed04433f3e178fef11ddacb2214c7e1864fb593c6f","0x743ab883fc143cc65ac7b084bd46104007a5ed18d5307a4740f6449bc9c8fa35","0x3b7c329f0d9ae75db9d7b1735fab9b2aefb783ad32de75deae44287fb8e2cba1","0x1a9854270d407c777756d0dcb164591a8dfb299d85f54490ffa30e7ed93e7261","0x556f37d8f3344c9148dae3528a7b10155ac6e3084fe84c147dc530a1f61d3feb","0x67041488dbfbd41e755ca09e5b8373efb271d409f15116279b3380493d6c65f0","0x9dd404db239e9dda6cb432b0b8b8eb9bb894edccafd3cb197a8a58e7bd0e03fc","0x8fd3f924dfc815a839246b1d88225bbaf52ec4a3fcccf5cac4beee94cd039e39","0x6cb059fc491d69b2f6544eb085c81b638dd7fb4234da57f4b9c5baa6c6cff333","0x508684d08392dbdd155ce7d99900d67b48c45b9ac9bca3171360ed0ddc0fd4aa","0x4e0eed10fb04e5fd155144618d8b184fa8c1dbf0a2af5b66cd04fc3d23f40e23","0xad35c37541b9e3f4e8863e0f129bf3459ce804e0bc02c598c63a2f05b74a9768","0xf6126e976a8f7952573e99653c3b33737b395a0e62568cb402a97c9ac31e4183","0x4181a698303a8dc682cf75a235159ed7d9aeda0e1ef10a4ed28e170814ddadc4","0xa106cd40e7fa518156cb698fa694065b818006acec0ab4e3e506a650eaaddcf3","0xb2d9488150a1f1c1bde436aba699de07d579902c0c2f5de509aa286de5e805b9","0x40cef523e2764f48042edc18b6c69af58c1c896b22044c789225e85767c2b535","0x08596204ee8d541c0cfb46e59e41a2f3b132056a22d1d6bcb6c04d3a70aa9645","0xf8cf70a1012cce64321b0a55ff1226abc4c94b99bc67d7586613cbc322a9ff8b","0x52da6bb94f43e4c2e110fbe7be6e252dbd490768cf16e0958d10e7d946a3122c","0x0c9e671fc6c7397e57dc94b161024858d53ec24c2d46c22c03d322e2c8c2e0d8","0x780ece6a99345a9d00a0522d821750d641bb79b8a4b37e7911b8c6663f794956","0x0713f84bb5bfa960299b65b07bfcf21c9b3e1e72c3a1cf810295fdaedfdb9263","0xc8e56d0ab6ea160c35703a053337e0f0c55579c19e35f43a3871b468bfaa2639","0x8023c5f75242a0b5ffa01bde4d37cfbe9231977c6b093ec5236af1a8254ac82e","0x56bc87e27159da613949cf85ac2a4dc9a2181ed89eff3abfab10d87b035abfd9","0x7585e8f7091fe0666b92905749b6301c8c68c2144543f40807856b77de7fbc1f","0x42855284973a6683b1f257024c0eedff05a3a79f0dc74b8d51b0b52bd68ce64d","0x3f9a5d6132ebd8e2b4448f856839f54dfaa764178d94aa261b94071f30a2ce27","0x2c24caeb6be5f86e3f4c3978ca6fdd819aebb38bd9b16206a02f68c9cc4beb3d","0xe8081efa007aaf320254fcacd851c76d179503beb81943dca132043c605437b9","0x51df5f33fbc435b8507b4b0313b12604a267d5b5191051ff8add3a95b0cd6eeb","0x87716aea37b2dbd6f2a99bea5f1f97d894e8e520096bb559f556f4e619666e65","0x6d491df47624408fdf672b4e9232854af15bcaf76838cd9e3a656dae1c5d291d","0x8e9f8fc07ed981d3a2afd0827efab70c981250a4cfbb195e7f8196748812e4e2","0x93b055ca83a59d77955f648906348eb66c056cba96891ba0bb9e68da05662b24","0xfe9c288cbe35d99104eb48abd1f78e66835d2c731c067e987a8438beda8ea775","0x4433272872d80025318b22e9e69c0d76000386165030e0cf70bf4b45b116db5d","0xe0c3c337aad9c431e602aef3b54fdce38405b834862bc35c8a1e891e2fc4df98","0xabc6ebec9c1cebce7de7563f40be437e8bde4619572aa3bac1815d97f6b16d27","0x5716d5d28f94ae531f05e1e2d55efa7211d7577563897a521865d0546dcb0b5f","0xac347c5f6345d6e7838f6c5bdad71c8503450b1bbad10901d673f4223dd6e90b","0xfed05b3ff206893610bfd157ded62813adc94f162dfa1af7d422f506e739efc6","0xc5161a3c9dcd8dedc69169e86d417474126dbeec80c7c984f876c1760475ae2c","0xcaa58964ebd4e40784eec25f2bacc3619698638772d9064d391a488338ac5718","0xd84cb49518f1887c7db9093f81a6d1573f2f0360ac41b9f172455a36f74547f9","0x69e91d095dcddc677bb0b1bf39a01bce235cc03368b0486f7eac52e23a4d5fa1","0xfd9b5b1b891412eeb8ba1ff95aa41757b787791857395bd1a8a75a30751e4992","0x1de640c7d35ea87b4a5003d931fdfc469ce7c10d6b067b785d3c81464f5deace","0x89fd8885a378cb4a61f4fe60e42cce75a83cdeba3f5b26f18ee384b417f7a9fd","0x5be968f6cd4c636bc8af7ba74c7bafa8180518bd729de8ee9c8028cbadeaa631","0x5129f99dbf37daee477170a8a46f88685518607ce4477abd97cf0baa7107dad2","0x60dea804c7d38d99b7eb14b5934e502db86fd6fb71ab8ed07e3915890bd7974d","0x0b21e1f9d75d3a8b7d6edd8b2ab96c208c55fbc7c82784daa86797947d303ad9","0xf28020e6cdc5309aa45cbb7a5c9853a23864c6dc788e137ff6c27618062ed48e","0x2a5cc1e6ac414070778bced67d4941b17794ed7a1374f90ba6056c41b299f0db","0xbce5dbff57bff610fdaa2f8823b39d8580727349a07a53adea037e43b5dc8217","0x4a1821f83bef65dd291c3e4d33ea6feac5fe1080aa96c0f66a477e0b4b440887","0x733d40284f372a0cb88d785cdd74d674e922b69316bc8bd5d0ca744cab4a23d4","0x07e56424d6f37e66ea6d395a2a3caf569474a9b3663eb10056af98cc43eef0e0","0xcb7f55de7324be9a768820793cba9baa0df9e7a8f32588b77d22b37dc55cc689","0x9396924612d6c2cbed6056dfc2a87b8a06b90c1246cf3b2f029c8320746a07b8","0x481012d8a7a45f59ac19eb868284e521f7d8be9dde208e46af438921794dc026","0x24d0ea0aec73d1cac82c447c91ab839aaa2ae2c2d4f8d7bafe803399455c16b8","0x4e44d03ebf7af4ba41e2794b95de933f081065c5fc2ec01002b60d01c4598150","0xf0f2af11a0d213f9a0ab081cd886849fe095242f80ae57497808fbd66fa3842e","0x7f0dbf6cb07fe6d387e7e509ad693247825bf2cd6b82ad44509dcb59937aa42b","0xd30ef99a403d2c2532c75ea3b6bb5f2e4e2e0a997107f5252dd426f933cc2945","0xe1129c0d6d98723fe131c0a309f35ae4db1c8c2cef829365295d54deb91caafb","0x27d1f971415b0aae2c50fc0baea7d62e9943abdebefe2d6ea3ed22f9fb37b156","0x0989ca72733bebe5173444b73221ed4d4f4234424ed375e50f595e6803d4558c","0x64ec7554242547f94f1c380139cab8e47ba6e93af2a1b25e2067b2a1ee2f1e50","0x7014429ec84bb6bdbfbac7f53de4386d6e8ed3b09df0bf99e25dbf094de54cea","0x98a653ba05eaeedfb85ef94e050dc9e6d031c5483f561a02ee3f63311be9f907","0x729cc84a2633aece645b6885a330f8cd378523c0e4ad705da2e4334a7d4a7c52"],"transactionsRoot":"0xdd0413a38dceb5eaaeb496ee4863a8c97d56f7b75cd5fcf555b3cdce62854bb7","uncles":[],"withdrawals":[{"address":"0xa8c62111e4652b07110a0fc81816303c42632f64","amount":"0xc43099","index":"0x4848c0","validatorIndex":"0x7ef3a"},{"address":"0x17f9d40f71e191434e53ab1b6a444c314d9e3c48","amount":"0xc42537","index":"0x4848c1","validatorIndex":"0x7ef3b"},{"address":"0xfdd04841960ed94e70967b216fc57519f3e5c338","amount":"0xc55a42","index":"0x4848c2","validatorIndex":"0x7ef3c"},{"address":"0xa281a6d58893f2a4259a5e9b65baec17595fa470","amount":"0xc49e61","index":"0x4848c3","validatorIndex":"0x7ef3d"},{"address":"0x2b78035514401ed1592eb691b8673a93edf97470","amount":"0xc5c6bb","index":"0x4848c4","validatorIndex":"0x7ef3e"},{"address":"0x2296e122c1a20fca3cac3371357bdad3be0df079","amount":"0xc29c58","index":"0x4848c5","validatorIndex":"0x7ef3f"},{"address":"0x2296e122c1a20fca3cac3371357bdad3be0df079","amount":"0xc4a07d","index":"0x4848c6","validatorIndex":"0x7ef40"},{"address":"0xc1e3fdda35577eca7ddbe4e6fd5082cb048e12bf","amount":"0xb946a8","index":"0x4848c7","validatorIndex":"0x7ef41"},{"address":"0x8be1b8cd5bedbea621d93a4c7f4c3e56b7d78fc6","amount":"0xc54ccf","index":"0x4848c8","validatorIndex":"0x7ef42"},{"address":"0x2b78035514401ed1592eb691b8673a93edf97470","amount":"0xc54e78","index":"0x4848c9","validatorIndex":"0x7ef43"},{"address":"0x5bc34a29600f545a5f0fed2c97306f1bdada035f","amount":"0xc52f4f","index":"0x4848ca","validatorIndex":"0x7ef44"},{"address":"0x8306300ffd616049fd7e4b0354a64da835c1a81c","amount":"0xc4caf5","index":"0x4848cb","validatorIndex":"0x7ef45"},{"address":"0xcf69a07ba10ba8fe7f691f2056ad77964c350e20","amount":"0xc5b6e2","index":"0x4848cc","validatorIndex":"0x7ef46"},{"address":"0x48176ca25402beb02711b8d890bd36950c39de74","amount":"0xc5b7ef","index":"0x4848cd","validatorIndex":"0x7ef47"},{"address":"0x6c9d84728161e4527af33ff32447adbfeebcc354","amount":"0xc55d41","index":"0x4848ce","validatorIndex":"0x7ef48"},{"address":"0x2b78035514401ed1592eb691b8673a93edf97470","amount":"0xc5f2b9","index":"0x4848cf","validatorIndex":"0x7ef49"}],"withdrawalsRoot":"0xfc0d4ba66394cb56297df7a4fec49b832980f1bfd980ab124542ba3abb8d83fa"}`,
				// eth_getBlockByNumber
				`{"baseFeePerGas":"0x7bf1725fc","difficulty":"0x0","extraData":"0x6265617665726275696c642e6f7267","gasLimit":"0x1c9c380","gasUsed":"0xbae2b1","hash":"0x9566b5caac7fd5aa96bc54d92d19c64e077b765c8bee15ddb854933c194ec4df","logsBloom":"0x02f3085241394140513b1800c009a7a5581428c82c41d9004045104d24621eaa39190923401001a10894d81242005b0902d98201a8d2b89c440084ac007aa95061c2e8180841fa7bae6b4009405c03a86404800689c20c51600adcc0d6848ce1dba40c22232a23202286561ac09408d1501950f21858260d800930f624f9843806500d40eddddd45644d08c60325111910089103298191f85421286a08920227ca4a0526640870214e8241d69ab905c004a462222023cc862961c520894c00e045bd28464000206834413800097a10004ac1062696d8489890813906c8d0784001b0e8ef2f11e00889a41a8000fb0b023526b98ae5ca17fc007a3938e0121c41","miner":"0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5","mixHash":"0x321b1389765b7bb4945da28b6f799fedbd824165479e4062f6ea167c9b40a2cd","nonce":"0x0000000000000000","number":"0x10877fb","parentHash":"0x334e0ebe85bddaac60dd8c22b840fc634ef3424a26db04ce48d612e7c890059f","receiptsRoot":"0x6be2d8781680201e0e5a06cf61f5c85e1731dee01a0c27ee351032d7ac69835a","sha3Uncles":"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347","size":"0xecb2","stateRoot":"0x2ee833ff79835d3b842abc965f6be7817e036132c5f74a4c579b55fbb61163af","timestamp":"0x646e998f","totalDifficulty":"0xc70d815d562d3cfa955","transactions":["0xc4a1cb5552c1f0e195cac76224417c33c7e33b84ac28db2a3ce5fec272756eae","0xfe86c32fe22b86131cbf7eb00702b49e1f3711767d53f9733b9599bbe20ed893","0xe515c1a14d7fddf96a5c87228a23f52da51e0e009d10530ad2fe0a847da872f4","0x5bb631aa823b87228512f49209aaae0c70bfdad6ad12c323e85168c02bc5495b","0x2639d1aca0d3fb3cb14690fca139d7c5e7afeccca2aeb85fce576ce7cb925f67","0x4ea0f719f09df279d393f43b1ceea3523cd2f77ea8903e48827d0ee653216c42","0x8956b7eb4f1ca53949bc05b33ea02ff2d9ad4422a3f49336a03e4390fc0e9687","0x90e6a7396df2e415723a7d3d7c82ab5e261a93556b601d4f992b1f903049b742","0xde4904dd6163251b4834d1385b9e2d321949d324314d489b4c366b88932b284a","0x04143f6b2800a378ac7fdd4ad45fd0370c7dfdd36ea4e68bac4f3ed757b47474","0x9aaac20be77b9360bd904e66ab1366b26deb7e45432a9b957c49e46c720c4eed","0x0c13be661af8459c78957cb9cba8c8c427e1291367168e52faf00592ea064305","0xd0750f5c2471fdce2d0b33bbf6874cf2fc511a28f98472c0032ccee91f8fc043","0x7749eb649d3a3e855b29934a2afa0ea0d869d078ef88d2d218e152f79f95fc8c","0xc5d0cd965d182fb64bb7dc1d74f34d37e0a1ac176e04862195799ab6b9eb904a","0xe74fb27cc46bb074e1acc17b8b1f5ab80f9606bdb2a85d667f873a82d52506b2","0xbb82f579f02c30069cc06354efe788a2d634f74bba067e968475221c32c5d7cc","0xac4f210a51bd2e6469a49f94f26fef7a29b43bbba0b3b8924e21013b831c7a65","0x27beaeb9a5eb99d27cc598bb0635befecbaebe05daa80bcc28bd1e7e24cbdab2","0x9fc072f9d84f7a767d1848ef9068ba10e57ed96412ff113ec2a22ee0a41e65f6","0x08fe755755a9c24b372c05071c9b4442cb3fe495af6e208ab671e54086af56ce","0x94222f57ef64c6528d2b4fca8fffb889ed18adb11afb62489b76ebf4a2888647","0xbedb1a5c7dbcd4c4a94f82f7c04070f5a25e09f21581b0c42bac20fe540d8033","0xe7c7a925f40d2ceefc6dce66ead6601a1c227e95107a56b4f8c8667f6271d672","0x1b1becada37cc4235bd42f3f5071c80a6c71c05900084ea94a2e1e25d0a5f2d6","0x56661b68b8fce950be4c959b7087d8cb46e3817441e33e7b570f32ff069ce0d8","0xb7e4885b38ba1a8fe1946dab388782196ca1538f7f4a8dc7258873ebd9066889","0xdc76a47b292495a5423fb9d342de5a355c165f458da95da89e347690737dab23","0xac0f12a3929f25506fa32040ae094afa6d3888e61fc3a33f14b37da2b6056593","0x817bab0bf326a261ee49bf18b30538353314eb2ec70b4d630c09c8aab179b822","0xe5deb67c07beee62e7aa908e7cc55f06bdfe86290b8e42cc89d5b18a5df0b314","0x4c9533e04c5cde31bfb4b054b68e744d95364c67904477cf459ae04bd113239c","0x3f21dbf23382e5d9bc74b9b586464ac203f1cc153de4c8e0647bea1d31697dcc","0x089a56f245ef562db50c336f74f33fea9a5b2d0f327e327a2b591ffbb1e2ccc0","0x8a2534510eae1f9df45f4d0f4c1abe3e1d67b8c69e6e1118d621e03c3fde75c6","0xa4bfec324533b934988c5a18d339db6cadd62f106642c4df2d5642da7e98a48b","0x68bd175b66fe52a75b8f4640d9801f8e36ee7a24c6342686cd8306f9b386a308","0x7c57a4c0ce0d1396f8bce84822bdfaa7536a071dcaa424096d522413fb627bea","0xbcd4fdcfc5d09b24c3162e7e66838c41ffb456ead1d951317e12389c59937e50","0x9bcd1fd924f77374db90024f8ec3b02b63be1dd6d75838f7cfaf9efedff95577","0x09cd0cf76ca4920386827dbed54c0d27cac8b989a9522baa972e7b474bcb2ff8","0x147a334fc806f09f1914f30d1a6c42db28510a753215a9802b7201a5920e441d","0x860f5498a54b5c2b8a44c0213826c26cab59f0447d29377804ef40028e9ddd17","0xc5042df5b1e3f7d5c8bde1a70108b2ea56af06623dc4dbc479cce374e9c6c792","0x201c43f27f763a9f52364225a9dbdd60562f59eae635aa369a18a6c3c410edb9","0x908c4bfe533c71751c8ec84185218919c1000d5e532da9911700dacb25fa5bc6","0x10a518c987fa84608559836192b2c25fdfdb9842525b2a90c7c8fd8314839b1e","0x769cddfdc9ff4d883406c6b237a67d71b7a9e47cb73e84b89c874e7345e48259","0x3e25aedb96ce0cf56e568fbff6f9fd85db9b1e9fd83afb4f760d4a7848e6368a","0xfe894c8429e6ca6beb1e4c4d05edef42d4291d3d1965f1bfded2254e9b831c47","0x2154ac9ad0dc175de759afe176419a32eb302dd2e3a8beba31e7f946379c0cf5","0x380e68ae7f3b5c33434e28f972a1b1c985dcf4502aca4242d4e9fd1c92678b86","0xd87fe7b356d600703f796c4b406a6d312a23c04cc78904cc4869f1f58be21168","0xd911140f5cd1a88ff1d69f24ad6a15626ac6210b69db281dc241227133de9b76","0x72466939fc323b939dc3d5ca53b2f650a0c0074cac334c1b210fb1ac60f9f45b","0x7776ce66a88077152152b1a42a08c3d3661b475949f97408c98e71718ea07479","0x28abb4e99a3689691dab5295d3b5a3d4ef896547c5868acd173a1755704ca9b1","0xd4c1e159e6bf41e3f14815bf133483862b650587c3e160943dc8b71659594834","0x56981da2684d277a66d82588494c2e3dcb8221f7835f1de27e02aeddc2c96fad","0x93dc972d039f3617ac7017fa3b4c74558a3b57594b31e4893a7a52fea93993a8","0x2ed28eb9c6585c4e6771f73e5eb7708ccadcd0bd73b4a9fcb8007ab26c73bb02","0x0dd4ee8f9858effa716ee5295c459088b9145023223761b5d3d221b647ce375b","0x5cdecf1c7121c3f699fb15669b7b5c25796768b7e35ffa8be66529c731eea9bb","0x22f497430f6f8665bdb5a3eae59a92070ae777a1439505c39dc8eebbb725e637","0x5941091e82c2980ff668bfd6c8b7a818e54ba6b8d0c075b53d6291d98f2632a1","0x37312bcbac83d9c25ae25b7faad0d5301e514c218979e1959609b9016b81c2e7","0x4e5f44552ec0b1143452613deeb8ad98300f3fd22b6c6afc1a3c41061d457fe7","0x5e7aef65bab1cf6c149a56b6cbba88f1beb10d4832b4b411a989f2aa7d17165c","0x6107f9aabea55a5577f6e7123a2ad71db26d32bc91685c6640cbcf16218a07c0","0x80764e04c7da8e6ac819d322940762e9a58eb9760d57f30ca87f96a2a4292013","0x1e0780ff446a41ef51eedfb1697a310a8b881983f62399b70259801e51a14f18","0x10b858b02933463e2935f29f43b4e4db0c0687cd54d52e281c2abfc34eae5c28","0xffe91eb4e21f3d6aeb4f98ac4027748dbafa9e991cddb71610cb21a8d5f94730","0xcabd9a07141a041367db082ecdc2cd4d4e3f7ba3bb03fe9e22b1039cb4759c81","0x1d62f981d86799305620ebf2aec19cfda5579c3de690d0720505a35ee4e6b905","0x98166c1e3368ebb6e582fd21ec996c779f35789c0a40f1f78b0bd09d038f849a","0x002d2af06e548bf59b3ccd102e582c4401858b2b9688d3e5714f830b13045594","0xa7cbafad30783caa700429dfb8cb4ee451f88c6f20793f14893c423a07402697","0xc52769b41c72badbf73f4bdc3fce5a48cfeac3eec8d9de6651e86edbbd97ad91","0x5dd188a27d101520c03bed22e07edd0b591eaf1cb43fe23324810c8fcb41ca88","0xab39d1bf987f7500fc89e192e045c99b2d2c0a6f7d8427cc36b8251741b3cffc","0xfa361812ffa5490bb738bcd66aea031e5e08de02257765a6cf76e10ac89a7119","0x887705092128ec3ea332db32895e5bb4c129f1284bd878d120c28e4960848708","0xe5170964f390d6f36d2a8380fb131c273e1a6c65d300d1af8ce4f64a5e8831a6","0x5abe3661f89538dae92ec4f503464cb12a732c80c1b47a5b0d9a923f4ef62228","0xf4c7fa7bf29f783c4c3de0fb686bf9dd27ced3fcff70c4d514197b382b908235","0x8b3d396ef0938ac8b733b0af7cd4eb829d9eeb6e1409ac381cbc292d261e476b","0xc2668d870aa758b2190d9f7477ac7becd4d823b5e3250ee2af802ebecbbbd0fb","0x98a622d45afb91982de9e58a846d831e8783f4e604b42774771f64521be0e75b","0x48111bc72433a6cf7b84d9b39f94dc861e40d3fe8e0b5a8c39297af299f6ba4a","0xe8101d8e48153c33cc7af912b20facc97bce6343b1e62af273e078b8c8cfe03f","0x408f5264d40ae56d9e7291906e7dddd45f183bbc47358fa17c0dfd83540a5cc2","0x9bec9972aa5c3efcd99d9fac717bb5cbcd26b1132e14ed79c7b3650dec9805f8","0xee858730ece22f6acc32e8a6dbff9a788dbaeee02d7fd5562b9fe5a9495fd1a9","0x60f558c76929ecfb5df205e2bce8491312def7875b5073fc76f9bf9bacc71ce6","0x783a72c195e6abd9aa0e7c7ae007d6479fd8998814f4363efbeb4f3201a9ffbd","0x276e1e505b36e351672aca08f8675b3621b9994157241eb4a2ae5001dc242412","0x7e08e21187caff3166ab6f2b94314d57a9113925452e7ec74220d54b799eb3c6","0x4d631bf187205de81f66c5b4502c4712592f274b9c2f2fcdb94208ea6364d3a3","0x835f74e59489a9e204b0ef194b1dd7cee20ba929fd98662f41ca89ae95eb92bf","0xb53a4552472e3d3521f32436a5a8cc9eb54237e2551b056f320a7af45b8c0b87","0xf6ccad6bafd0fb3746c97c5144f22b0505aa05eb43d7d1eb8f11d821c5b67446","0x508270cc0346b37993895cfa7bd63949e95d7a95b0d73ec16032f3692c50366c","0xf1c56dcc48ad89247b9d178ae8f52cdebefb2daf4bca3f28410ed3b3b310378b","0xe9b5b8abb207b3aea964462f9c152244958ff60e6e3933a4f3b6ffcd22309841","0x9cb408af23f16380f871741ea83c59459339dc7d224e30b89c9c8f3e27800173","0x5ff51b3a4ae1d34181ec14ebff8490f76a3164f7563dceaa017382485d578e10","0xef630e08b727e0f183091650af01e04ebd0a343995197d07b67576391727d443","0x6f960b58a598f394a32998140b75a1d7b2d5ab96a3bd898d1d3897a5e4146e2b","0x885de73efe238ba7741a6897c17f6ceaad19a98313a4f2623cf195ec8c15e9aa","0xc7a9242a48b25031ab4b32a3928c29c083850ae04f3f17d86b73ee2856f4e119","0x975388de837313c0a0bd84589645616c89003c4d238e469282972cd129c74a79","0x74dacc09901dcecd611db70f49bcef8630866a77a1a2478979d77223afe7c4e4","0xccd1162597118995d86183b2c937d3984e4ab602ffe84f92cd5bab2c6000c0b3","0xfd7e3819c8bd29f3d768aba92eba8b61b8876d433a1dbb8e6b95fa028a4ee74a","0xaf5d2187bab5cefa3c688d99053d9c45ed0b91b389ddd6e3fcdef219219e520a","0xc9129ee17b19a1a442b858b9592f9fd6932d0dead52c9b9ef41a1e733ecec1d1","0x90e9f17eefac406b9c06366fe54ad9176af0643724d5fdba0325341571e2b152","0x938d4bb8e1a57ddadc9b0d718288c2da7cd4c77b0bc26efd70c833b4c2391aff","0xbf2dfe13a390584112790c49a494c196b4901b71e240859b371539be3bfba997","0xcb3a364ae94b984d0593d75fda35ec74982c81fe8b8246037c524c7c32e7438e","0x358e96ac20dc91c72e5da544c7cbd706e42e5b29f8891e0e698a8ebeef8f5542","0x5e03225399f65632f483cfd265db8af57e9bfdd616dc2677fc8d868889d2047f","0x9d6dea2caec631d04f1298a761f2b32167ebd44b003ca2eeb651a6cfab748e58","0x381fe3f309ebe4c8a0748b193132d962835484ce86d6c4efc50569896651e1ce","0xfb7a6a95b2de1eea1edec7d0d5b5351c54566c7d46f9b864b0beea6895defbbd","0x7c68ac4ca34f9458a5ad27858a6bf493c286c25fb740d6a4628a5d40e8132052","0x39890f39d507115a62335de7a82fed5acbf0a1f977320cdeb4bb3d8af4718860","0xca4ef0934f75c4b6151563cbe1d5be1256b272d68727646800be4b0d8f3e7538","0x42d5a505d62dcc1ab3023d8d4c7378b958156975b47dc33bdc81375b1d785950","0x05fb92283322910929aae49f2a1187fba4997fe96f0b5ce91b15437311b8a05b","0xf634431645d3b813b395861f281c2a9d4631bf15d842e6b88cd5eb20321f0435","0xfa3cdf0f1413f5718ff16b0538d9ff8ca5b0da2e030478f64bdd1461b252da34","0x1721f612c729dfb1d1ede6336018243f687cc24ab5ae12beb30b3266b3c35833","0xc15e343b6aa406a4e9ddf6a23f033203fd69db07634e925ea2b55609546e53fc","0xee071b09ebfb987b084c6989633acc29b92208cec0f7539e09d1d36cac753de1","0x6120cedd1adc05d0e74beb17d627e15be3a6e1209acc8e1df3c53678fff0654d"],"transactionsRoot":"0x01c4d79f44aca14e329ff3aa49e19c622d813e20c7ca683ccad45f331d3f90e4","uncles":[],"withdrawals":[{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc528d2","index":"0x489760","validatorIndex":"0x840c3"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc47626","index":"0x489761","validatorIndex":"0x840c4"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc45321","index":"0x489762","validatorIndex":"0x840c5"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc5402c","index":"0x489763","validatorIndex":"0x840c6"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc5244b","index":"0x489764","validatorIndex":"0x840c7"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc54a88","index":"0x489765","validatorIndex":"0x840c8"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc549d7","index":"0x489766","validatorIndex":"0x840c9"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc4b7e1","index":"0x489767","validatorIndex":"0x840ca"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc531b3","index":"0x489768","validatorIndex":"0x840cb"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0x2cdf809","index":"0x489769","validatorIndex":"0x840cc"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc4f455","index":"0x48976a","validatorIndex":"0x840cd"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc560f5","index":"0x48976b","validatorIndex":"0x840ce"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc552e5","index":"0x48976c","validatorIndex":"0x840cf"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc58495","index":"0x48976d","validatorIndex":"0x840d0"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc4c1c6","index":"0x48976e","validatorIndex":"0x840d1"},{"address":"0xb9d7934878b5fb9610b3fe8a5e441e8fad7e293f","amount":"0xc47d6d","index":"0x48976f","validatorIndex":"0x840d2"}],"withdrawalsRoot":"0xf25074ba63e9847671dd46b132cb7ea06440ed086663e30be68296aa3d34ad66"}`,
			},
			xc.TxInfo{
				BlockHash:   "0x17d3a092ad1855a468fd1cbfeb03245ab09367272daf2433aa93ebb41e570f2d",
				TxID:        "b3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
				ExplorerURL: "/tx/0xb3dcb32a7bb4856845898033522c676c1d2d50e0b07e5ec36880cd2d8b2a6b0f",
				From:        "",
				// to is now the contract address of contract making multiple transfers
				To:              "0xA9D1e08C7793af67e9d92fe308d5697FB81d3E43",
				BlockIndex:      17330961,
				BlockTime:       1684954511,
				Confirmations:   1258,
				ContractAddress: "0xA9D1e08C7793af67e9d92fe308d5697FB81d3E43",
				Sources: []*xc.TxInfoEndpoint{
					{
						Address: "0xA9D1e08C7793af67e9d92fe308d5697FB81d3E43",
					},
				},
				Destinations: []*xc.TxInfoEndpoint{
					{
						Address: "0xC1f0E841572CCFc0B85A84F4Ab532571701085d5",
						Amount:  xc.NewAmountBlockchainFromStr("24366840000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x95bc53640B38AB48132CBFd3B058Fe625B40F55f",
						Amount:  xc.NewAmountBlockchainFromStr("25129770000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0xFdb0171A63795629b27ca715Ffd09c85e82c37Be",
						Amount:  xc.NewAmountBlockchainFromStr("27917440000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0xD20Fbde842dDBE5110a64f6e7B45D33Fca0D78b2",
						Amount:  xc.NewAmountBlockchainFromStr("48409740000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x085A07FF3FA9635de9771dc60f8C37d2f12c05f0",
						Amount:  xc.NewAmountBlockchainFromStr("51912490000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x4368e1B4CE65eaC80D1c1F063071265e5D851466",
						Amount:  xc.NewAmountBlockchainFromStr("82489210000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x352232369Aa3b2C551a47fe16Ed1052Bb624dfd3",
						Amount:  xc.NewAmountBlockchainFromStr("99457820000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x55786C2009E833c198F43F7793aE6d9C7D14D31C",
						Amount:  xc.NewAmountBlockchainFromStr("100000000000000000"),
						Asset:   "ETH",
					},

					{
						Address: "0x89D079BbaAF3FC0ceAfb94Df622104892027c332",
						Amount:  xc.NewAmountBlockchainFromStr("103284450000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x6112456a34FF9b583985Ab3cC7125bbc2b7B17f6",
						Amount:  xc.NewAmountBlockchainFromStr("139620730000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0xA1CD502F9c91b4fcd234d83A5d63585A2FDbadBF",
						Amount:  xc.NewAmountBlockchainFromStr("164739590000000000"),
						Asset:   "ETH",
					},
					{
						Address: "0x00007d0BA516a2bA02D77907d3a1348C1187Ae62",
						Amount:  xc.NewAmountBlockchainFromStr("4630000000000000000"),
						Asset:   "ETH",
					},
				},
				Fee: xc.NewAmountBlockchainFromStr("6231934410218064"),
				// amount is the first destination
			},
			"",
		},
	}

	for _, v := range vectors {
		fmt.Println("testing ", v.name)
		server, close := testtypes.MockJSONRPC(&s.Suite, v.resp)
		defer close()
		asset := &xc.AssetConfig{NativeAsset: xc.ETH, Net: "testnet", URL: server.URL, ChainID: 5}

		asset.URL = server.URL
		client, _ := NewClient(asset)
		txInfo, err := client.FetchTxInfo(s.Ctx, xc.TxHash(v.txHash))

		if v.err != "" {
			require.Equal(xc.TxInfo{}, txInfo)
			require.ErrorContains(err, v.err)
		} else {
			require.Nil(err)
			require.NotNil(txInfo)
			require.Equal(v.val, txInfo)
		}
	}
}
