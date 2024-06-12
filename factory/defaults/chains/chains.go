package chains

import (
	_ "embed"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/common"
)

func init() {
	maincfg := common.Unmarshal(mainnetData)
	testcfg := common.Unmarshal(testnetData)

	Mainnet = maincfg.Chains
	Testnet = testcfg.Chains

	for _, chain := range Mainnet {
		if chain.Net == "" {
			chain.Net = maincfg.Network
		}
		if chain.ConfirmationsFinal == 0 {
			chain.ConfirmationsFinal = 6
		}

		// default to using xc client
		defaultUrl := "https://connector.cordialapis.com"
		chain.Clients = []*xc.ClientConfig{
			{
				Driver: xc.DriverCrosschain,
				URL:    defaultUrl,
			},
		}
	}
	for _, chain := range Testnet {
		if chain.Net == "" {
			chain.Net = testcfg.Network
		}
		if chain.ConfirmationsFinal == 0 {
			chain.ConfirmationsFinal = 2
		}
		defaultUrl := "https://connector-testnet.cordialapis.com"
		chain.Clients = []*xc.ClientConfig{
			{
				Driver: xc.DriverCrosschain,
				URL:    defaultUrl,
			},
		}
	}
}

//go:embed mainnet.yaml
var mainnetData string

//go:embed testnet.yaml
var testnetData string

var Mainnet map[string]*xc.ChainConfig
var Testnet map[string]*xc.ChainConfig
