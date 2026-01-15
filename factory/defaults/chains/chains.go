package chains

import (
	_ "embed"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/common"
)

//go:embed mainnet.yaml
var mainnetData string

//go:embed testnet.yaml
var testnetData string

func init() {
	maincfg := common.Unmarshal(mainnetData)
	testcfg := common.Unmarshal(testnetData)

	Mainnet = maincfg.Chains
	Testnet = testcfg.Chains

	for _, chain := range Mainnet {
		if chain.Network == "" {
			chain.Network = string(maincfg.Network)
		}
		if chain.XConfirmationsFinal == 0 {
			chain.XConfirmationsFinal = 6
		}
		if chain.Confirmations.Final == 0 {
			chain.Confirmations.Final = 6
		}
		if chain.CrosschainClient.Network == "" {
			chain.CrosschainClient.Network = xc.Mainnets
		}
	}
	for key, chain := range Testnet {
		if chain.FeeLimit.String() == "0" {
			// clone the mainnet value
			mainnetConfig, ok := Mainnet[key]
			if ok {
				chain.FeeLimit, _ = xc.NewAmountHumanReadableFromStr(mainnetConfig.FeeLimit.String())
			}
		}

		if chain.Network == "" {
			chain.Network = string(testcfg.Network)
		}
		if chain.XConfirmationsFinal == 0 {
			chain.XConfirmationsFinal = 2
		}
		if chain.Confirmations.Final == 0 {
			chain.Confirmations.Final = 2
		}
		if chain.CrosschainClient.Network == "" {
			chain.CrosschainClient.Network = xc.NotMainnets
		}
	}
}

var Mainnet map[string]*xc.ChainConfig
var Testnet map[string]*xc.ChainConfig
