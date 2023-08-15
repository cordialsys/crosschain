package config_test

import (
	"fmt"
	"os"
	"testing"

	xc "github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/config"
	"github.com/jumpcrypto/crosschain/config/constants"
	"github.com/jumpcrypto/crosschain/factory"
	factoryconfig "github.com/jumpcrypto/crosschain/factory/config"
	"github.com/jumpcrypto/crosschain/factory/defaults"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
)

type CrosschainTestSuite struct {
	suite.Suite
}

func (s *CrosschainTestSuite) SetupTest() {
}
func TestFactoryConfig(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func (s *CrosschainTestSuite) TestAssetUnmarshal() {
	require := s.Require()
	var cfg factoryconfig.Config
	err := yaml.Unmarshal([]byte(`
  chains:
    ATOM:
      asset: ATOM
      driver: cosmos
      net: testnet
      url: 'myurl'
      chain_id_str: 'theta-testnet-001'
      chain_prefix: 'cosmos'
      chain_coin: 'uatom'
      chain_coin_hd_path: 118
      chain_name: Cosmos
      explorer_url: 'myexplorer'
      decimals: 6
    SOL:
      asset: SOL
      driver: solana
      net: mainnet
      url: 'https://api.devnet.solana.com'
      chain_name: Solana
      explorer_url: 'https://explorer.solana.com'
      decimals: 9

  tokens:
    USDC.SOL:
      asset: USDC
      chain: SOL
      net: testnet
      decimals: 6
      contract: 4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU

  tasks:
    # Solana
    sol-wrap:
      name: sol-wrap
      default_params:
        param1: abc
      code: WrapTx
      chain: SOL
      allow:
      - SOL -> WSOL.SOL
    sol-unwrap:
      name: sol-unwrap
      default_params:
        param2: xyz
      code: UnwrapEverythingTx
      chain: SOL
      allow:
      - WSOL.SOL -> SOL

  pipelines:
    wrappyMcUnwrappyFace:
      name: wrappyMcUnwrappyFace
      allow:
        - SOL -> WSOL.SOL
        - WSOL.SOL -> SOL
        - ETH -> WETH.ETH
        - WETH.ETH->ETH
      tasks:
        - sol-wrap
        - sol-unwrap

`), &cfg)
	require.NoError(err)

	bz, err := yaml.Marshal(&cfg)
	require.NoError(err)
	cfg = factoryconfig.Config{}
	err = yaml.Unmarshal(bz, &cfg)
	require.NoError(err)

	// Test tokens and chains
	require.Len(cfg.Chains, 2)
	require.Len(cfg.Tokens, 1)
	require.Len(cfg.GetChainsAndTokens(), 3)

	// viper lowercases the config keys, but yaml natively
	// is case sensitive.
	require.Equal("ATOM", cfg.Chains["ATOM"].Asset)
	require.Equal("Cosmos", cfg.Chains["ATOM"].ChainName)
	require.Equal("SOL", cfg.Chains["SOL"].Asset)
	require.Equal("Solana", cfg.Chains["SOL"].ChainName)

	require.Equal("USDC", cfg.Tokens["USDC.SOL"].Asset)
	require.Equal("USDC", cfg.Tokens["USDC.SOL"].AssetConfig.Asset)

	tasks := cfg.GetTasks()
	pipelines := cfg.GetPipelines()

	// Test tasks
	require.Len(tasks, 2)
	// Allow lists should be parsed
	require.Len(tasks[0].AllowList, 1)
	require.Len(tasks[1].AllowList, 1)
	require.Equal(tasks[0].AllowList[0], &xc.AllowEntry{Src: "WSOL.SOL", Dst: "SOL"})
	require.Equal(tasks[1].AllowList[0], &xc.AllowEntry{Src: "SOL", Dst: "WSOL.SOL"})
	require.Contains(tasks[0].DefaultParams, "param2")
	require.Equal(tasks[0].DefaultParams["param2"], "xyz")
	require.Contains(tasks[1].DefaultParams, "param1")
	require.Equal(tasks[1].DefaultParams["param1"], "abc")
	// Test pipelines
	require.Len(pipelines, 1)
	require.Equal(pipelines[0].Name, "wrappyMcUnwrappyFace")
	require.Len(pipelines[0].AllowList, 4)
	require.Equal(pipelines[0].AllowList[0], &xc.AllowEntry{Src: "SOL", Dst: "WSOL.SOL"})
	require.Equal(pipelines[0].AllowList[3], &xc.AllowEntry{Src: "WETH.ETH", Dst: "ETH"})
}

type ConfigWrapper struct {
	factoryconfig.Config `yaml:"crosschain"`
}

func (s *CrosschainTestSuite) TestConfigFullyLoads() {
	require := s.Require()
	cfgBz := []byte(`
crosschain:
  chains:
    ETH:
      asset: ETH
      driver: evm
      net: testnet
      url: 'https://goerli.infura.io/v3'
      auth: 'env:INFURA_API_TOKEN'
      provider: infura
      chain_id: 5
      chain_name: Ethereum (Goerli)
      explorer_url: 'https://goerli.etherscan.io'
      decimals: 18
  tasks:
    wormhole-transfer:
      name: wormhole-transfer
      code: WormholeTransferTx
      default_params:
        arbiter_fee_usd: 5
      operations:
      - function: transfer
        signature: 0f5287b0
        contract:
          ETH: 0x3ee18B2214AFF97000D974cf647E7C347E8fa585
          FTM: 0x7C9Fc5741288cDFdD83CeB07f3ea7e22618D79D2
          AVAX: 0x0e082F06FF657D94310cB8cE8B0D9a04541d8052
        params:
        - name: token
          type: address
          bind: contract
        - name: amount
          type: uint256
          bind: amount
        - name: chain
          type: uint256
          match: dst_asset
          value:
            SOL: 1
            ETH: 2
            MATIC: 5
        - name: recipient
          type: address
          bind: to
`)
	file, _ := os.CreateTemp(os.TempDir(), "xctest")
	file.Write(cfgBz)

	os.Setenv(constants.ConfigEnv, file.Name())
	defer os.Unsetenv(constants.ConfigEnv)
	var wrapper ConfigWrapper
	wrapper.Parse()
	yaml.Unmarshal(cfgBz, &wrapper)
	require.Contains(wrapper.Config.Tasks["wormhole-transfer"].DefaultParams, "arbiter_fee_usd")
	var cfg factoryconfig.Config
	err := config.RequireConfig("crosschain", &cfg, nil)
	require.NoError(err)
	cfg.Parse()

	// Test tokens and chains
	require.Len(cfg.Chains, 1)
	require.Len(cfg.Tasks, 1)
	require.Len(cfg.GetChainsAndTokens(), 1)
	require.Contains(cfg.Tasks["wormhole-transfer"].DefaultParams, "arbiter_fee_usd")
}

func (s *CrosschainTestSuite) TestUseMainnet() {
	require := s.Require()
	cfgBz := []byte(`
crosschain:
  network: mainnet
  chains:
    ETH:
      url: 'myurl'
`)
	file, _ := os.CreateTemp(os.TempDir(), "xctest")
	file.Write(cfgBz)
	os.Setenv(constants.ConfigEnv, file.Name())

	xcf := factory.NewFactory(&factory.FactoryOptions{
		UseDisabledChains: true,
	})
	expectedChainCount := len(defaults.Mainnet.Chains)
	count := 0
	xcf.AllAssets.Range(func(key, value any) bool {
		if chain, ok := value.(*xc.NativeAssetConfig); ok {
			count += 1
			require.NotEqual(chain.Net, "testnet")
			require.NotEqual(chain.Net, "")
			require.NotEqual(chain.Net, "devnet")
		}
		return true
	})
	require.Equal(expectedChainCount, count)
}

func (s *CrosschainTestSuite) TestMergeWitDefaults() {

	pw := "1234"
	os.Setenv("TEST_PASSWORD", pw)
	require := s.Require()
	type testcase struct {
		cfg            string
		expectedAssets int
		expectedUrl    string
		expectedAuth   string
	}
	for i, tc := range []testcase{
		{
			cfg: `
crosschain:
  chains:
    ETH:
      url: myurl
      auth: 'env:TEST_PASSWORD'
`,
			// should stay the same
			expectedAssets: len(defaults.Testnet.Chains) + len(defaults.Testnet.Tokens),
			expectedUrl:    "myurl",
			expectedAuth:   pw,
		},
		{
			cfg: `
crosschain:
  chains:
    eth:
      url: myurl2
`,
			// should stay the same
			expectedAssets: len(defaults.Testnet.Chains) + len(defaults.Testnet.Tokens),
			expectedUrl:    "myurl2",
		},
		{
			cfg: `
crosschain:
  chains:
    eth:
      url: myurl3
    eth123:
      asset: eth123
      url: myurl4
`,
			// should be 1 extra chain
			expectedAssets: 1 + len(defaults.Testnet.Chains) + len(defaults.Testnet.Tokens),
			expectedUrl:    "myurl3",
		},
	} {
		fmt.Println("testcase ", i)
		file, _ := os.CreateTemp(os.TempDir(), "xctest")
		file.Write([]byte(tc.cfg))
		os.Setenv(constants.ConfigEnv, file.Name())
		f := factory.NewDefaultFactory()
		count := 0
		f.AllAssets.Range(func(key, val any) bool {
			count += 1
			return true
		})
		require.Equal(tc.expectedAssets, count, "there is likely a token or chain with duplicate identifier")

		eth, err := f.GetAssetConfig("", "ETH")
		require.NoError(err)
		require.Equal("ETH", eth.GetNativeAsset().Asset)
		require.Equal(tc.expectedAuth, eth.GetNativeAsset().AuthSecret)
		require.Equal(tc.expectedUrl, eth.GetNativeAsset().URL)

	}

}

func (s *CrosschainTestSuite) TestSorted() {
	require := s.Require()
	// run multiple times since go map keys are not deterministic
	for i := 0; i < 10; i++ {
		cfg := factoryconfig.Config{
			Chains: map[string]*xc.NativeAssetConfig{
				"BBB": {
					Asset: "BBB",
				},
				"AAA": {
					Asset: "AAA",
				},
				"CCC": {
					Asset: "CCC",
				},
			},
			Tasks: map[string]*xc.TaskConfig{
				"BBB": {
					Name: "BBB",
				},
				"AAA": {
					Name: "AAA",
				},
				"CCC": {
					Name: "CCC",
				},
			},
			Pipelines: map[string]*xc.PipelineConfig{
				"BBB": {
					Name: "BBB",
				},
				"AAA": {
					Name: "AAA",
				},
				"CCC": {
					Name: "CCC",
				},
			},
		}
		assets := cfg.GetChainsAndTokens()
		tasks := cfg.GetTasks()
		pipes := cfg.GetChainsAndTokens()
		require.Len(assets, 3)
		require.Equal("AAA", string(assets[0].ID()))
		require.Equal("BBB", string(assets[1].ID()))
		require.Equal("CCC", string(assets[2].ID()))

		require.Len(tasks, 3)
		require.Equal("AAA", string(tasks[0].ID()))
		require.Equal("BBB", string(tasks[1].ID()))
		require.Equal("CCC", string(tasks[2].ID()))

		require.Len(pipes, 3)
		require.Equal("AAA", string(pipes[0].ID()))
		require.Equal("BBB", string(pipes[1].ID()))
		require.Equal("CCC", string(pipes[2].ID()))

	}
}
