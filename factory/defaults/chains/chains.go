package chains

import (
	_ "embed"

	xc "github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/factory/defaults/common"
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
		if chain.URL == "" && len(chain.Clients) == 0 {
			chain.Clients = append(chain.Clients, &xc.ClientConfig{
				Driver: string(xc.DriverCrosschain),
				URL:    "https://crosschain.cordialapis.com",
			})
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

var Mainnet map[string]*xc.NativeAssetConfig
var Testnet map[string]*xc.NativeAssetConfig
