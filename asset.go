package crosschain

import (
	"fmt"
	"strings"
)

type SignatureType string

const (
	K256    = SignatureType("k256")
	Ed255   = SignatureType("ed255")
	Schnorr = SignatureType("schnorr")
)

// NativeAsset is an asset on a blockchain used to pay gas fees.
// In Crosschain, for simplicity, a NativeAsset represents a chain.
type NativeAsset string

// List of supported NativeAsset
const (
	// UTXO
	BCH  = NativeAsset("BCH")  // Bitcoin Cash
	BTC  = NativeAsset("BTC")  // Bitcoin
	DOGE = NativeAsset("DOGE") // Dogecoin
	LTC  = NativeAsset("LTC")  // Litecoin

	// Account-based
	ACA    = NativeAsset("ACA")    // Acala
	APTOS  = NativeAsset("APTOS")  // APTOS
	ArbETH = NativeAsset("ArbETH") // Arbitrum
	ATOM   = NativeAsset("ATOM")   // Cosmos
	AurETH = NativeAsset("AurETH") // Aurora
	AVAX   = NativeAsset("AVAX")   // Avalanche
	BNB    = NativeAsset("BNB")    // Binance Coin
	CELO   = NativeAsset("CELO")   // Celo
	CHZ    = NativeAsset("CHZ")    // Chiliz
	CHZ2   = NativeAsset("CHZ2")   // Chiliz 2.0
	DOT    = NativeAsset("DOT")    // Polkadot
	ETC    = NativeAsset("ETC")    // Ethereum Classic
	ETH    = NativeAsset("ETH")    // Ethereum
	ETHW   = NativeAsset("ETHW")   // Ethereum PoW
	FTM    = NativeAsset("FTM")    // Fantom
	HASH   = NativeAsset("HASH")   // Provenance
	INJ    = NativeAsset("INJ")    // Injective
	LUNA   = NativeAsset("LUNA")   // Terra V2
	LUNC   = NativeAsset("LUNC")   // Terra Classic
	KAR    = NativeAsset("KAR")    // Karura
	KLAY   = NativeAsset("KLAY")   // Klaytn
	XDC    = NativeAsset("XDC")    // XinFin
	MATIC  = NativeAsset("MATIC")  // Polygon
	OAS    = NativeAsset("OAS")    // Oasys (not Oasis!)
	OptETH = NativeAsset("OptETH") // Optimism
	EmROSE = NativeAsset("EmROSE") // Rose (Oasis EVM-compat "Emerald" parachain)
	SOL    = NativeAsset("SOL")    // Solana
	SUI    = NativeAsset("SUI")    // SUI
	XPLA   = NativeAsset("XPLA")   // XPLA
	TIA    = NativeAsset("TIA")    // celestia
	SEI    = NativeAsset("SEI")    // Sei
)

var NativeAssetList []NativeAsset = []NativeAsset{
	BCH,
	BTC,
	DOGE,
	LTC,
	ACA,
	APTOS,
	ArbETH,
	ATOM,
	AurETH,
	AVAX,
	BNB,
	CELO,
	CHZ,
	CHZ2,
	DOT,
	ETC,
	ETH,
	ETHW,
	FTM,
	INJ,
	HASH,
	LUNA,
	LUNC,
	KAR,
	KLAY,
	XDC,
	MATIC,
	OAS,
	OptETH,
	EmROSE,
	SOL,
	SUI,
	XPLA,
	TIA,
	SEI,
}

// Driver is the type of a chain
type Driver string

// List of supported Driver
const (
	DriverAptos       = Driver("aptos")
	DriverBitcoin     = Driver("bitcoin")
	DriverCosmos      = Driver("cosmos")
	DriverCosmosEvmos = Driver("evmos")
	DriverEVM         = Driver("evm")
	DriverEVMLegacy   = Driver("evm-legacy")
	DriverSubstrate   = Driver("substrate")
	DriverSolana      = Driver("solana")
	DriverSui         = Driver("sui")
	// Crosschain is a client-only driver
	DriverCrosschain = Driver("crosschain")
)

var SupportedDrivers = []Driver{
	DriverAptos,
	DriverBitcoin,
	DriverCosmos,
	DriverCosmosEvmos,
	DriverEVM,
	DriverEVMLegacy,
	DriverSubstrate,
	DriverSolana,
	DriverSui,
}

func (native NativeAsset) IsValid() bool {
	return NativeAsset(native).Driver() != ""
}

func (native NativeAsset) Driver() Driver {
	switch native {
	case BCH, BTC, DOGE, LTC:
		return DriverBitcoin
	case AVAX, CELO, ETH, ETHW, MATIC, OptETH, ArbETH:
		return DriverEVM
	case BNB, FTM, ETC, EmROSE, AurETH, ACA, KAR, KLAY, OAS, CHZ, XDC, CHZ2:
		return DriverEVMLegacy
	case APTOS:
		return DriverAptos
	case ATOM, XPLA, INJ, HASH, LUNC, LUNA, SEI, TIA:
		return DriverCosmos
	case SUI:
		return DriverSui
	case SOL:
		return DriverSolana
	case DOT:
		return DriverSubstrate
	}
	return ""
}

func (driver Driver) SignatureAlgorithm() SignatureType {
	switch driver {
	case DriverBitcoin, DriverEVM, DriverEVMLegacy, DriverCosmos, DriverCosmosEvmos:
		return K256
	case DriverAptos, DriverSolana, DriverSui:
		return Ed255
	case DriverSubstrate:
		return Schnorr
	}
	return ""
}

// AssetID is an internal identifier for each asset
// Examples: ETH, USDC, USDC.SOL - see tests for details
type AssetID string

// ClientConfig is the model used to represent a client inside an AssetConfig
type ClientConfig struct {
	Driver   string `yaml:"driver"`
	URL      string `yaml:"url,omitempty"`
	Auth     string `yaml:"auth,omitempty"`
	Provider string `yaml:"provider,omitempty"`
}

// AssetConfig is the model used to represent an asset read from config file or db
type NativeAssetConfig struct {
	Asset                string          `yaml:"asset,omitempty"`
	Driver               string          `yaml:"driver,omitempty"`
	Net                  string          `yaml:"net,omitempty"`
	Clients              []*ClientConfig `yaml:"clients,omitempty"`
	URL                  string          `yaml:"url,omitempty"`
	FcdURL               string          `yaml:"fcd_url,omitempty"`
	Auth                 string          `yaml:"auth,omitempty"`
	Provider             string          `yaml:"provider,omitempty"`
	ChainID              int64           `yaml:"chain_id,omitempty"`
	ChainIDStr           string          `yaml:"chain_id_str,omitempty"`
	ChainName            string          `yaml:"chain_name,omitempty"`
	ChainPrefix          string          `yaml:"chain_prefix,omitempty"`
	ChainCoin            string          `yaml:"chain_coin,omitempty"`
	GasCoin              string          `yaml:"gas_coin,omitempty"`
	ChainCoinHDPath      uint32          `yaml:"chain_coin_hd_path,omitempty"`
	ChainGasPriceDefault float64         `yaml:"chain_gas_price_default,omitempty"`
	ChainGasMultiplier   float64         `yaml:"chain_gas_multiplier,omitempty"`
	ChainGasTip          uint64          `yaml:"chain_gas_tip,omitempty"`
	ChainMaxGasPrice     float64         `yaml:"chain_max_gas_price,omitempty"`
	ChainTransferTax     float64         `yaml:"chain_transfer_tax,omitempty"`
	ExplorerURL          string          `yaml:"explorer_url,omitempty"`
	Decimals             int32           `yaml:"decimals,omitempty"`
	IndexerUrl           string          `yaml:"indexer_url,omitempty"`
	IndexerType          string          `yaml:"indexer_type,omitempty"`
	IndexerSymbol        string          `yaml:"indexer_symbol,omitempty"`
	PollingPeriod        string          `yaml:"polling_period,omitempty"`
	NoGasFees            bool            `yaml:"no_gas_fees,omitempty"`
	Disabled             *bool           `yaml:"disabled,omitempty"`

	// Internal
	// dereferenced api token if used
	AuthSecret string `yaml:"-"`
}

type TokenAssetConfig struct {
	Asset    string `yaml:"asset,omitempty"`
	Chain    string `yaml:"chain,omitempty"`
	Decimals int32  `yaml:"decimals,omitempty"`
	Contract string `yaml:"contract,omitempty"`

	// Token configs are joined with a chain config upon loading.
	// If there is no matching native asset config, there will be a loading error.
	NativeAssetConfig *NativeAssetConfig `yaml:"-"`
}

// type AssetMetadataConfig struct {
// 	PriceUSD AmountHumanReadable `yaml:"-"`
// }

var _ ITask = &NativeAssetConfig{}
var _ ITask = &TokenAssetConfig{}

func (c NativeAssetConfig) String() string {
	// do NOT print AuthSecret
	return fmt.Sprintf(
		"NativeAssetConfig(id=%s asset=%s chainId=%d driver=%s chainCoin=%s prefix=%s net=%s url=%s auth=%s provider=%s)",
		c.ID(), c.Asset, c.ChainID, c.Driver, c.ChainCoin, c.ChainPrefix, c.Net, c.URL, c.Auth, c.Provider,
	)
}

func (asset *NativeAssetConfig) ID() AssetID {
	return GetAssetIDFromAsset("", asset.Asset)
}

func (asset *NativeAssetConfig) GetDecimals() (int32, bool) {
	return asset.Decimals, true
}

// func (asset NativeAssetConfig) GetDriver() Driver {
// 	return Driver(asset.Driver)
// }

func (asset *NativeAssetConfig) GetNativeAsset() *NativeAssetConfig {
	return asset
}

func (native *NativeAssetConfig) GetContract() (string, bool) {
	return "", false
}

// TODO we should delete these extra fields that are indicative of chain
// func (asset *NativeAssetConfig) GetChainIdentifier() string {
// 	return asset.Asset
// }

// func (asset NativeAssetConfig) GetTask() *TaskConfig {
// 	return nil
// }

// Return list of clients with the "default" client added
// if it's not already there
func (asset NativeAssetConfig) GetAllClients() []*ClientConfig {
	defaultCfg := &ClientConfig{
		Driver: asset.Driver,
		URL:    asset.URL,
	}
	cfgs := asset.Clients[:]
	hasDefault := false
	for _, cfg := range cfgs {
		if cfg.Driver == defaultCfg.Driver {
			hasDefault = true
		}
	}
	empty := defaultCfg.Driver == "" && defaultCfg.URL == ""
	if !hasDefault && !empty {
		cfgs = append(cfgs, defaultCfg)
	}

	return cfgs
}

// Return all clients that are not crosschain driver
func (asset NativeAssetConfig) GetNativeClients() []*ClientConfig {
	clients := asset.GetAllClients()
	filtered := []*ClientConfig{}
	for _, client := range clients {
		if client.Driver != string(DriverCrosschain) {
			filtered = append(filtered, client)
		}
	}
	return filtered
}

func (native *NativeAssetConfig) GetAssetSymbol() (string, bool) {
	return native.Asset, true
}

func (c *TokenAssetConfig) String() string {
	net := ""
	native := c.GetNativeAsset()
	if native != nil {
		net = native.Net
	}
	return fmt.Sprintf(
		"TokenAssetConfig(id=%s asset=%s chain=%s net=%s decimals=%d contract=%s)",
		c.ID(), c.Asset, c.Chain, net, c.Decimals, c.Contract,
	)
}

func (asset *TokenAssetConfig) ID() AssetID {
	return GetAssetIDFromAsset(asset.Asset, asset.Chain)
}

func (asset *TokenAssetConfig) GetNativeAsset() *NativeAssetConfig {
	return asset.NativeAssetConfig
}

//	func (asset *TokenAssetConfig) GetDriver() Driver {
//		return Driver(asset.GetNativeAsset().Driver)
//	}
func (asset *TokenAssetConfig) GetDecimals() (int32, bool) {
	return asset.Decimals, true
}

func (token *TokenAssetConfig) GetContract() (string, bool) {
	return token.Contract, true
}
func (token *TokenAssetConfig) GetAssetSymbol() (string, bool) {
	return token.Asset, true
}

// func (asset *TokenAssetConfig) GetAssetConfig() *AssetConfig {
// 	asset.AssetConfig.Asset = asset.Asset
// 	asset.AssetConfig.Chain = asset.Chain
// 	asset.AssetConfig.Net = asset.Net
// 	asset.AssetConfig.Decimals = asset.Decimals
// 	asset.AssetConfig.Contract = asset.Contract
// 	asset.AssetConfig.Type = asset.Type
// 	return &asset.AssetConfig
// }

// func (asset *TokenAssetConfig) GetTask() *TaskConfig {
// 	return nil
// }

func parseAssetAndNativeAsset(asset string, nativeAsset string) (string, string) {
	if asset == "" && nativeAsset == "" {
		return "", ""
	}
	if asset == "" && nativeAsset != "" {
		asset = nativeAsset
	}

	assetSplit := strings.Split(asset, ".")
	if len(assetSplit) == 2 && NativeAsset(assetSplit[1]).IsValid() {
		asset = assetSplit[0]
		if nativeAsset == "" {
			nativeAsset = assetSplit[1]
		}
	}
	validNative := NativeAsset(asset).IsValid()

	if nativeAsset == "" {
		if validNative {
			nativeAsset = asset
		} else {
			nativeAsset = "ETH"
		}
	}

	return asset, nativeAsset
}

// GetAssetIDFromAsset return the canonical AssetID given two input strings asset, nativeAsset.
// Input can come from user input.
// Examples:
// - GetAssetIDFromAsset("USDC", "") -> "USDC.ETH"
// - GetAssetIDFromAsset("USDC", "ETH") -> "USDC.ETH"
// - GetAssetIDFromAsset("USDC", "SOL") -> "USDC.SOL"
// - GetAssetIDFromAsset("USDC.SOL", "") -> "USDC.SOL"
// See tests for more examples.
func GetAssetIDFromAsset(asset string, nativeAsset string) AssetID {
	// id is SYMBOL for ERC20 and SYMBOL.CHAIN for others
	// e.g. BTC, ETH, USDC, SOL, USDC.SOL
	asset, nativeAsset = parseAssetAndNativeAsset(asset, nativeAsset)
	validNative := NativeAsset(asset).IsValid()

	// native asset, e.g. BTC, ETH, SOL
	if asset == nativeAsset {
		return AssetID(asset)
	}
	if nativeAsset == "ETH" && !validNative {
		return AssetID(asset + ".ETH")
	}
	// token, e.g. USDC, USDC.SOL
	return AssetID(asset + "." + nativeAsset)
}
