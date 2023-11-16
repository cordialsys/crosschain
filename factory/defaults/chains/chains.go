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

		// default to using xc client
		defaultUrl := "https://crosschain.cordialapis.com"
		if chain.URL == "" && len(chain.Clients) == 0 {
			chain.Clients = append(chain.Clients, &xc.ClientConfig{
				Driver: xc.DriverCrosschain,
				URL:    defaultUrl,
			})
		}
		for _, client := range chain.Clients {
			if client.Driver == xc.DriverCrosschain && client.URL == "" {
				client.URL = defaultUrl
			}
		}
	}
	for _, chain := range Testnet {
		if chain.Net == "" {
			chain.Net = testcfg.Network
		}
		// no testnet support in xc service yet
	}
}

//go:embed mainnet.yaml
var mainnetData string

//go:embed testnet.yaml
var testnetData string

var Mainnet map[string]*xc.ChainConfig
var Testnet map[string]*xc.ChainConfig
