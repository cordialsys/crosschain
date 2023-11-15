package tokens

import (
	_ "embed"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/defaults/common"
)

func init() {
	maincfg := common.Unmarshal(mainnetData)
	testcfg := common.Unmarshal(testnetData)

	Mainnet = maincfg.Tokens
	Testnet = testcfg.Tokens

	// for _, chain := range Mainnet {
	// 	if chain.Net == "" {
	// 		chain.Net = maincfg.Network
	// 	}
	// }
	// for _, chain := range Testnet {
	// 	if chain.Net == "" {
	// 		chain.Net = testcfg.Network
	// 	}
	// }
}

//go:embed mainnet.yaml
var mainnetData string

//go:embed testnet.yaml
var testnetData string

var Mainnet map[string]*xc.TokenAssetConfig
var Testnet map[string]*xc.TokenAssetConfig
