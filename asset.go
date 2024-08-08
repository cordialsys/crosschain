package crosschain

import (
	"fmt"
	"slices"
	"strings"
)

type SignatureType string

const (
	K256Keccak = SignatureType("k256-keccak")
	K256Sha256 = SignatureType("k256-sha256")
	Ed255      = SignatureType("ed255")
	Schnorr    = SignatureType("schnorr")
)

// NativeAsset is an asset on a blockchain used to pay gas fees.
// In Crosschain, for simplicity, a NativeAsset represents a chain.
type NativeAsset string

// List of supported NativeAsset
const (
	ACA    = NativeAsset("ACA")    // Acala
	APTOS  = NativeAsset("APTOS")  // APTOS
	ArbETH = NativeAsset("ArbETH") // Arbitrum
	ATOM   = NativeAsset("ATOM")   // Cosmos
	AurETH = NativeAsset("AurETH") // Aurora
	AVAX   = NativeAsset("AVAX")   // Avalanche
	BERA   = NativeAsset("BERA")   // Berachain
	BCH    = NativeAsset("BCH")    // Bitcoin Cash
	BNB    = NativeAsset("BNB")    // Binance Coin
	BTC    = NativeAsset("BTC")    // Bitcoin
	CELO   = NativeAsset("CELO")   // Celo
	CHZ    = NativeAsset("CHZ")    // Chiliz
	CHZ2   = NativeAsset("CHZ2")   // Chiliz 2.0
	DOGE   = NativeAsset("DOGE")   // Dogecoin
	DOT    = NativeAsset("DOT")    // Polkadot
	ETC    = NativeAsset("ETC")    // Ethereum Classic
	ETH    = NativeAsset("ETH")    // Ethereum
	ETHW   = NativeAsset("ETHW")   // Ethereum PoW
	FTM    = NativeAsset("FTM")    // Fantom
	HASH   = NativeAsset("HASH")   // Provenance
	INJ    = NativeAsset("INJ")    // Injective
	LTC    = NativeAsset("LTC")    // Litecoin
	LUNA   = NativeAsset("LUNA")   // Terra V2
	LUNC   = NativeAsset("LUNC")   // Terra Classic
	KAR    = NativeAsset("KAR")    // Karura
	KLAY   = NativeAsset("KLAY")   // Klaytn
	KSM    = NativeAsset("KSM")    // Kusama
	XDC    = NativeAsset("XDC")    // XinFin
	MATIC  = NativeAsset("MATIC")  // Polygon
	OAS    = NativeAsset("OAS")    // Oasys (not Oasis!)
	OptETH = NativeAsset("OptETH") // Optimism
	EmROSE = NativeAsset("EmROSE") // Rose (Oasis EVM-compat "Emerald" parachain)
	SOL    = NativeAsset("SOL")    // Solana
	SUI    = NativeAsset("SUI")    // SUI
	XPLA   = NativeAsset("XPLA")   // XPLA
	TAO    = NativeAsset("TAO")    // Bittensor
	TIA    = NativeAsset("TIA")    // celestia
	TON    = NativeAsset("TON")    // TON
	TRX    = NativeAsset("TRX")    // TRON
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
	BERA,
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
	KSM,
	XDC,
	MATIC,
	OAS,
	OptETH,
	EmROSE,
	SOL,
	SUI,
	XPLA,
	TAO,
	TIA,
	TON,
	TRX,
	SEI,
}

// Driver is the type of a chain
type Driver string

// List of supported Driver
const (
	DriverAptos         = Driver("aptos")
	DriverBitcoin       = Driver("bitcoin")
	DriverBitcoinCash   = Driver("bitcoin-cash")
	DriverBitcoinLegacy = Driver("bitcoin-legacy")
	DriverCosmos        = Driver("cosmos")
	DriverCosmosEvmos   = Driver("evmos")
	DriverEVM           = Driver("evm")
	DriverEVMLegacy     = Driver("evm-legacy")
	DriverSubstrate     = Driver("substrate")
	DriverSolana        = Driver("solana")
	DriverSui           = Driver("sui")
	DriverTron          = Driver("tron")
	DriverTon           = Driver("ton")
	// Crosschain is a client-only driver
	DriverCrosschain = Driver("crosschain")
)

var SupportedDrivers = []Driver{
	DriverAptos,
	DriverBitcoin,
	DriverBitcoinCash,
	DriverBitcoinLegacy,
	DriverCosmos,
	DriverCosmosEvmos,
	DriverEVM,
	DriverEVMLegacy,
	DriverSubstrate,
	DriverSolana,
	DriverSui,
	DriverTron,
	DriverTon,
}

type StakingProvider string

const Kiln StakingProvider = "kiln"
const Figment StakingProvider = "figment"
const Twinstake StakingProvider = "twinstake"
const Native StakingProvider = "native"

var SupportedStakingProviders = []StakingProvider{
	Native,
	Kiln,
	Figment,
	Twinstake,
}

func (stakingProvider StakingProvider) Valid() bool {
	return slices.Contains(SupportedStakingProviders, stakingProvider)
}

type TxVariantInputType string

func NewStakingInputType(driver Driver, variant string) TxVariantInputType {
	return TxVariantInputType(fmt.Sprintf("drivers/%s/staking/%s", driver, variant))
}

func NewUnstakingInputType(driver Driver, variant string) TxVariantInputType {
	return TxVariantInputType(fmt.Sprintf("drivers/%s/unstaking/%s", driver, variant))
}

func NewWithdrawingInputType(driver Driver, variant string) TxVariantInputType {
	return TxVariantInputType(fmt.Sprintf("drivers/%s/withdrawing/%s", driver, variant))
}

func (variant TxVariantInputType) Driver() Driver {
	return Driver(strings.Split(string(variant), "/")[1])
}
func (variant TxVariantInputType) Variant() string {
	return (strings.Split(string(variant), "/")[3])
}

func (variant TxVariantInputType) Validate() error {
	if len(strings.Split(string(variant), "/")) != 4 {
		return fmt.Errorf("invalid input variant type: %s", variant)
	}
	return nil
}

func (native NativeAsset) IsValid() bool {
	return NativeAsset(native).Driver() != ""
}

func (native NativeAsset) Driver() Driver {
	switch native {
	case BTC:
		return DriverBitcoin
	case BCH:
		return DriverBitcoinCash
	case DOGE, LTC:
		return DriverBitcoinLegacy
	case AVAX, CELO, ETH, ETHW, MATIC, OptETH, ArbETH, BERA:
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
	case DOT, TAO, KSM:
		return DriverSubstrate
	case TRX:
		return DriverTron
	case TON:
		return DriverTon
	}
	return ""
}

func (driver Driver) SignatureAlgorithm() SignatureType {
	switch driver {
	case DriverBitcoin, DriverBitcoinCash, DriverBitcoinLegacy:
		return K256Sha256
	case DriverEVM, DriverEVMLegacy, DriverCosmos, DriverCosmosEvmos, DriverTron:
		return K256Keccak
	case DriverAptos, DriverSolana, DriverSui, DriverTon, DriverSubstrate:
		return Ed255
	}
	return ""
}

type PublicKeyFormat string

var Raw PublicKeyFormat = "raw"
var Compressed PublicKeyFormat = "compressed"
var Uncompressed PublicKeyFormat = "uncompressed"

func (driver Driver) PublicKeyFormat() PublicKeyFormat {
	switch driver {
	case DriverBitcoin, DriverBitcoinCash, DriverBitcoinLegacy:
		return Compressed
	case DriverCosmos, DriverCosmosEvmos:
		return Compressed
	case DriverEVM, DriverEVMLegacy, DriverTron:
		return Uncompressed
	case DriverAptos, DriverSolana, DriverSui, DriverTon, DriverSubstrate:
		return Raw
	}
	return ""
}

// AssetID is an internal identifier for each asset (legacy/deprecated)
// Examples: ETH, USDC, USDC.SOL - see tests for details
type AssetID string

// Network selector is used by crosschain client to select which network of a blockchain to select.
type NetworkSelector string

const Mainnets NetworkSelector = ""
const NotMainnets NetworkSelector = "!mainnet"

// ClientConfig is the model used to represent a client inside an AssetConfig
type ClientConfig struct {
	Driver   Driver          `yaml:"driver"`
	URL      string          `yaml:"url,omitempty"`
	Auth     string          `yaml:"auth,omitempty"`
	Provider string          `yaml:"provider,omitempty"`
	Network  NetworkSelector `yaml:"network,omitempty"`
}

type ExplorerUrls struct {
	Tx      string `yaml:"tx"`
	Address string `yaml:"address"`
	Token   string `yaml:"token"`
}

type StakingConfig struct {
	// the contract used for staking, if relevant
	StakeContract string `yaml:"stake_contract,omitempty"`
	// the contract used for unstaking, if relevant
	UnstakeContract string `yaml:"unstake_contract,omitempty"`
	// Compatible providers for staking
	Providers []StakingProvider `yaml:"providers,omitempty"`
}

func (staking *StakingConfig) Enabled() bool {
	return len(staking.Providers) > 0
}

// AssetConfig is the model used to represent an asset read from config file or db
type ChainConfig struct {
	Chain                NativeAsset     `yaml:"chain,omitempty"`
	XAssetDeprecated     NativeAsset     `yaml:"asset,omitempty"` // deprecated
	Driver               Driver          `yaml:"driver,omitempty"`
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
	ChainMinGasPrice     float64         `yaml:"chain_min_gas_price,omitempty"`
	ChainTransferTax     float64         `yaml:"chain_transfer_tax,omitempty"`
	ExplorerURL          string          `yaml:"explorer_url,omitempty"`
	Decimals             int32           `yaml:"decimals,omitempty"`
	IndexerUrl           string          `yaml:"indexer_url,omitempty"`
	IndexerType          string          `yaml:"indexer_type,omitempty"`
	IndexerSymbol        string          `yaml:"indexer_symbol,omitempty"`
	PollingPeriod        string          `yaml:"polling_period,omitempty"`
	NoGasFees            bool            `yaml:"no_gas_fees,omitempty"`
	Disabled             *bool           `yaml:"disabled,omitempty"`
	ExplorerUrls         ExplorerUrls    `yaml:"explorer_urls"`

	ConfirmationsFinal int `yaml:"confirmations_final,omitempty"`

	Staking StakingConfig `yaml:"staking,omitempty"`

	// Internal
	// dereferenced api token if used
	AuthSecret string `yaml:"-"`
}

type TokenAssetConfig struct {
	Asset    string      `yaml:"asset,omitempty"`
	Chain    NativeAsset `yaml:"chain,omitempty"`
	Decimals int32       `yaml:"decimals,omitempty"`
	Contract string      `yaml:"contract,omitempty"`

	// Token configs are joined with a chain config upon loading.
	// If there is no matching native asset config, there will be a loading error.
	ChainConfig *ChainConfig `yaml:"-"`
}

// type AssetMetadataConfig struct {
// 	PriceUSD AmountHumanReadable `yaml:"-"`
// }

var _ ITask = &ChainConfig{}
var _ ITask = &TokenAssetConfig{}

func (c ChainConfig) String() string {
	// do NOT print AuthSecret
	return fmt.Sprintf(
		"NativeAssetConfig(id=%s asset=%s chainId=%d driver=%s chainCoin=%s prefix=%s net=%s url=%s auth=%s provider=%s)",
		c.ID(), c.Chain, c.ChainID, c.Driver, c.ChainCoin, c.ChainPrefix, c.Net, c.URL, c.Auth, c.Provider,
	)
}

func (asset *ChainConfig) ID() AssetID {
	return GetAssetIDFromAsset("", asset.Chain)
}

func (asset *ChainConfig) GetDecimals() int32 {
	return asset.Decimals
}

// func (asset NativeAssetConfig) GetDriver() Driver {
// 	return Driver(asset.Driver)
// }

func (asset *ChainConfig) GetChain() *ChainConfig {
	return asset
}

func (native *ChainConfig) GetContract() string {
	return ""
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
func (asset ChainConfig) GetAllClients() []*ClientConfig {
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
func (asset ChainConfig) GetNativeClients() []*ClientConfig {
	clients := asset.GetAllClients()
	filtered := []*ClientConfig{}
	for _, client := range clients {
		if client.Driver != DriverCrosschain {
			filtered = append(filtered, client)
		}
	}
	return filtered
}

func (native *ChainConfig) GetAssetSymbol() string {
	return string(native.Chain)
}

func (c *TokenAssetConfig) String() string {
	net := ""
	native := c.GetChain()
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

func (asset *TokenAssetConfig) GetChain() *ChainConfig {
	return asset.ChainConfig
}

//	func (asset *TokenAssetConfig) GetDriver() Driver {
//		return Driver(asset.GetNativeAsset().Driver)
//	}
func (asset *TokenAssetConfig) GetDecimals() int32 {
	return asset.Decimals
}

func (token *TokenAssetConfig) GetContract() string {
	return token.Contract
}
func (token *TokenAssetConfig) GetAssetSymbol() string {
	return token.Asset
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

func LegacyParseAssetAndNativeAsset(asset string, nativeAsset string) (string, NativeAsset) {
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

	return asset, NativeAsset(nativeAsset)
}

// GetAssetIDFromAsset return the canonical AssetID given two input strings asset, nativeAsset.
// Input can come from user input.
// Examples:
// - GetAssetIDFromAsset("USDC", "") -> "USDC.ETH"
// - GetAssetIDFromAsset("USDC", "ETH") -> "USDC.ETH"
// - GetAssetIDFromAsset("USDC", "SOL") -> "USDC.SOL"
// - GetAssetIDFromAsset("USDC.SOL", "") -> "USDC.SOL"
// See tests for more examples.
func GetAssetIDFromAsset(asset string, nativeAsset NativeAsset) AssetID {
	// id is SYMBOL for ERC20 and SYMBOL.CHAIN for others
	// e.g. BTC, ETH, USDC, SOL, USDC.SOL
	asset, nativeAsset = LegacyParseAssetAndNativeAsset(asset, string(nativeAsset))
	validNative := NativeAsset(asset).IsValid()

	// native asset, e.g. BTC, ETH, SOL
	if asset == string(nativeAsset) {
		return AssetID(asset)
	}
	if nativeAsset == "ETH" && !validNative {
		return AssetID(asset + ".ETH")
	}
	// token, e.g. USDC, USDC.SOL
	return AssetID(asset + "." + string(nativeAsset))
}
