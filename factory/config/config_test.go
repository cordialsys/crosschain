package config_test

import (
	"fmt"
	"os"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/config/constants"
	"github.com/cordialsys/crosschain/factory"
	factoryconfig "github.com/cordialsys/crosschain/factory/config"
	"github.com/cordialsys/crosschain/factory/defaults"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type CrosschainTestSuite struct {
	suite.Suite
}

func (s *CrosschainTestSuite) SetupTest() {
}
func TestFactoryConfig(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

func (s *CrosschainTestSuite) TestChainUnmarshal() {
	require := s.Require()
	var cfg factoryconfig.Config
	err := yaml.Unmarshal([]byte(`
  chains:
    ATOM:
      chain: ATOM
      driver: cosmos
      net: testnet
      url: 'myurl'
      chain_id: 'theta-testnet-001'
      chain_prefix: 'cosmos'
      chain_coin: 'uatom'
      chain_coin_hd_path: 118
      chain_name: Cosmos
      explorer_url: 'myexplorer'
      decimals: 6
      fee_limit: "0.0001"
    SOL:
      chain: SOL
      driver: solana
      net: mainnet
      url: 'https://api.devnet.solana.com'
      chain_name: Solana
      explorer_url: 'https://explorer.solana.com'
      decimals: 9
      fee_limit: "100.0"
`), &cfg)
	require.NoError(err)

	bz, err := yaml.Marshal(&cfg)
	require.NoError(err)
	fmt.Println("re-marshaled:")
	fmt.Println(string(bz))
	cfg = factoryconfig.Config{}
	err = yaml.Unmarshal(bz, &cfg)
	require.NoError(err)
	cfg.MigrateFields()

	// Test tokens and chains
	require.Len(cfg.Chains, 2)

	// viper lowercases the config keys, but yaml natively
	// is case sensitive.
	require.Equal(xc.ATOM, cfg.Chains["ATOM"].Chain)
	require.Equal("Cosmos", cfg.Chains["ATOM"].ChainName)
	require.Equal("0.0001", cfg.Chains["ATOM"].FeeLimit.String())

	require.Equal(xc.SOL, cfg.Chains["SOL"].Chain)
	require.Equal("Solana", cfg.Chains["SOL"].ChainName)
	require.Equal("100", cfg.Chains["SOL"].FeeLimit.String())
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

`)
	file, _ := os.CreateTemp(os.TempDir(), "xctest")
	_, err := file.Write(cfgBz)
	require.NoError(err)

	err = os.Setenv(constants.ConfigEnv, file.Name())
	require.NoError(err)
	defer os.Unsetenv(constants.ConfigEnv)
	var wrapper ConfigWrapper
	wrapper.Parse()
	err = yaml.Unmarshal(cfgBz, &wrapper)
	require.NoError(err)
	var cfg factoryconfig.Config
	err = config.RequireConfig("crosschain", &cfg, nil)
	require.NoError(err)
	cfg.Parse()

	// Test tokens and chains
	require.Len(cfg.Chains, 1)
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
	_, err := file.Write(cfgBz)
	require.NoError(err)
	os.Setenv(constants.ConfigEnv, file.Name())

	xcf := factory.NewFactory(&factory.FactoryOptions{
		UseDisabledChains: true,
	})
	expectedChainCount := len(defaults.Mainnet.Chains)
	count := 0
	for _, chain := range xcf.AllChains {
		count += 1
		require.NotEqual(chain.Network, "testnet")
		require.NotEqual(chain.Network, "")
		require.NotEqual(chain.Network, "devnet")
	}
	require.Equal(expectedChainCount, count)
}

func (s *CrosschainTestSuite) TestMergeWitDefaults() {

	pw := "1234"
	os.Setenv("TEST_PASSWORD", pw)
	require := s.Require()
	type testcase struct {
		cfg             string
		expectedAssets  int
		expectedUrl     string
		expectedAuth    string
		expectedNetwork string
	}
	for i, tc := range []testcase{
		{
			cfg: `
crosschain:
  network: testnet
  chains:
    ETH:
      url: myurl
      auth: 'env:TEST_PASSWORD'
`,
			// should stay the same
			expectedAssets:  len(defaults.Testnet.Chains),
			expectedUrl:     "myurl",
			expectedAuth:    pw,
			expectedNetwork: "testnet",
		},
		{
			cfg: `
crosschain:
  network: testnet
  chains:
    eth:
      url: myurl2
`,
			// should stay the same
			expectedAssets:  len(defaults.Testnet.Chains),
			expectedUrl:     "myurl2",
			expectedNetwork: "testnet",
		},
		{
			cfg: `
crosschain:
  network: mainnet
  chains:
    eth:
      url: myurl_mainnet
`,
			// should have mainnet assets
			expectedAssets:  len(defaults.Mainnet.Chains),
			expectedUrl:     "myurl_mainnet",
			expectedNetwork: "mainnet",
		},
		{
			cfg: `
crosschain:
  chains:
    eth:
      url: myurl_mainnet
`,
			// should default to mainnet
			expectedAssets:  len(defaults.Mainnet.Chains),
			expectedUrl:     "myurl_mainnet",
			expectedNetwork: "mainnet",
		},
		{
			cfg: `
crosschain:
  network: testnet
  chains:
    eth:
      url: myurl3
    eth123:
      asset: eth123
      url: myurl4
`,
			// should be 1 extra chain
			expectedAssets:  1 + len(defaults.Testnet.Chains),
			expectedUrl:     "myurl3",
			expectedNetwork: "testnet",
		},
	} {
		fmt.Println("testcase ", i)
		file, _ := os.CreateTemp(os.TempDir(), "xctest")
		_, err := file.Write([]byte(tc.cfg))
		require.NoError(err)
		err = os.Setenv(constants.ConfigEnv, file.Name())
		require.NoError(err)
		f := factory.NewFactory(&factory.FactoryOptions{
			UseDisabledChains: true,
		})
		count := 0
		for range f.AllChains {
			count += 1
		}

		require.EqualValues(tc.expectedNetwork, f.Config.Network)
		require.Equal(tc.expectedAssets, count, "there is likely a token or chain with duplicate identifier")
		eth, ok := f.GetChain("ETH")
		require.True(ok)
		require.Equal(xc.ETH, eth.GetChain().Chain)

		var secret string
		if eth.GetChain().Auth2 != "" {
			secret, err = eth.GetChain().Auth2.Load()
			require.NoError(err)
		}
		require.Equal(tc.expectedAuth, secret)
		require.Equal(tc.expectedUrl, eth.GetChain().URL)

	}

}

func (s *CrosschainTestSuite) TestSorted() {
	require := s.Require()
	// run multiple times since go map keys are not deterministic
	for i := 0; i < 10; i++ {
		cfg := factoryconfig.Config{
			Chains: map[string]*xc.ChainConfig{
				"BBB": xc.NewChainConfig("BBB"),
				"AAA": xc.NewChainConfig("AAA"),
				"CCC": xc.NewChainConfig("CCC"),
			},
		}
		chains := cfg.GetChains()
		require.Len(chains, 3)
		require.EqualValues("AAA", string(chains[0].Chain))
		require.EqualValues("BBB", string(chains[1].Chain))
		require.EqualValues("CCC", string(chains[2].Chain))

	}
}
