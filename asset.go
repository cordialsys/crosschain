package crosschain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cordialsys/crosschain/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type SignatureType string

const (
	K256Keccak        = SignatureType("k256-keccak")
	K256Sha256        = SignatureType("k256-sha256")
	Ed255             = SignatureType("ed255")
	Schnorr           = SignatureType("schnorr")
	Bls12_381G2Blake2 = SignatureType("bls12-381-g2-blake2")
)

// NativeAsset is an asset on a blockchain used to pay gas fees.
// In Crosschain, for simplicity, a NativeAsset represents a chain.
type NativeAsset string

// List of supported NativeAsset
const (
	ACA    = NativeAsset("ACA")    // Acala
	ADA    = NativeAsset("ADA")    // Cardano
	AKT    = NativeAsset("AKT")    // Akash
	APTOS  = NativeAsset("APTOS")  // APTOS
	ArbETH = NativeAsset("ArbETH") // Arbitrum
	ASTR   = NativeAsset("ASTR")   // Astar
	ATOM   = NativeAsset("ATOM")   // Cosmos
	AurETH = NativeAsset("AurETH") // Aurora
	AVAX   = NativeAsset("AVAX")   // Avalanche
	BAND   = NativeAsset("BAND")   // Band
	BASE   = NativeAsset("BASE")   // BASE
	BABY   = NativeAsset("BABY")   // Babylon
	BERA   = NativeAsset("BERA")   // Berachain
	BCH    = NativeAsset("BCH")    // Bitcoin Cash
	BNB    = NativeAsset("BNB")    // Binance Coin
	BTC    = NativeAsset("BTC")    // Bitcoin
	CELO   = NativeAsset("CELO")   // Celo
	CHZ    = NativeAsset("CHZ")    // Chiliz
	CHZ2   = NativeAsset("CHZ2")   // Chiliz 2.0
	DOGE   = NativeAsset("DOGE")   // Dogecoin
	DOT    = NativeAsset("DOT")    // Polkadot
	DUSK   = NativeAsset("DUSK")   // Dusk
	ENJ    = NativeAsset("ENJ")    // Enjin
	EOS    = NativeAsset("EOS")    // EOS
	ETC    = NativeAsset("ETC")    // Ethereum Classic
	ETH    = NativeAsset("ETH")    // Ethereum
	ETHW   = NativeAsset("ETHW")   // Ethereum PoW
	FIL    = NativeAsset("FIL")    // Filecoin
	FTM    = NativeAsset("FTM")    // Fantom
	HASH   = NativeAsset("HASH")   // Provenance
	INJ    = NativeAsset("INJ")    // Injective
	LTC    = NativeAsset("LTC")    // Litecoin
	LUNA   = NativeAsset("LUNA")   // Terra V2
	LUNC   = NativeAsset("LUNC")   // Terra Classic
	KAR    = NativeAsset("KAR")    // Karura
	KAS    = NativeAsset("KAS")    // Kaspa
	KAVA   = NativeAsset("KAVA")   // Kava
	KLAY   = NativeAsset("KLAY")   // Klaytn
	KSM    = NativeAsset("KSM")    // Kusama
	XDC    = NativeAsset("XDC")    // XinFin
	MATIC  = NativeAsset("MATIC")  // Polygon
	MON    = NativeAsset("MON")    // MONAD
	NOBLE  = NativeAsset("NOBLE")  // Noble Chain
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
	SeiEVM = NativeAsset("SeiEVM") // SeiEVM
	XRP    = NativeAsset("XRP")    // XRP
	XLM    = NativeAsset("XLM")    // XLM
	ZETA   = NativeAsset("ZETA")   // ZetaChain
	NIL    = NativeAsset("NIL")    // Nillion
)

var NativeAssetList []NativeAsset = []NativeAsset{
	ADA,
	AKT,
	BAND,
	BABY,
	BCH,
	BTC,
	DOGE,
	LTC,
	ACA,
	APTOS,
	ArbETH,
	ASTR,
	ATOM,
	AurETH,
	AVAX,
	BERA,
	BNB,
	CELO,
	CHZ,
	CHZ2,
	DOT,
	DUSK,
	ENJ,
	EOS,
	ETC,
	ETH,
	ETHW,
	FIL,
	FTM,
	INJ,
	HASH,
	LUNA,
	LUNC,
	KAR,
	KAS,
	KAVA,
	KLAY,
	KSM,
	XDC,
	MATIC,
	MON,
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
	SeiEVM,
	XRP,
	BASE,
	NOBLE,
	XLM,
	ZETA,
	NIL,
}

// Driver is the type of a chain
type Driver string

// List of supported Driver
const (
	DriverAptos         = Driver("aptos")
	DriverBitcoin       = Driver("bitcoin")
	DriverBitcoinCash   = Driver("bitcoin-cash")
	DriverBitcoinLegacy = Driver("bitcoin-legacy")
	DriverCardano       = Driver("cardano")
	DriverCosmos        = Driver("cosmos")
	DriverCosmosEvmos   = Driver("evmos")
	DriverDusk          = Driver("dusk")
	DriverEOS           = Driver("eos")
	DriverEVM           = Driver("evm")
	DriverEVMLegacy     = Driver("evm-legacy")
	DriverFilecoin      = Driver("filecoin")
	DriverKaspa         = Driver("kaspa")
	DriverSubstrate     = Driver("substrate")
	DriverSolana        = Driver("solana")
	DriverSui           = Driver("sui")
	DriverTron          = Driver("tron")
	DriverTon           = Driver("ton")
	DriverXrp           = Driver("xrp")
	DriverXlm           = Driver("xlm")
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
	DriverEOS,
	DriverEVM,
	DriverEVMLegacy,
	DriverFilecoin,
	DriverKaspa,
	DriverSubstrate,
	DriverSolana,
	DriverSui,
	DriverTron,
	DriverTon,
	DriverXrp,
	DriverXlm,
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

func NewMultiTransferInputType(driver Driver, variant string) TxVariantInputType {
	return TxVariantInputType(fmt.Sprintf("drivers/%s/multi-transfer/%s", driver, variant))
}

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
	case AVAX, BNB, CELO, ETH, ETHW, MATIC, OptETH, ArbETH, BERA, BASE, SeiEVM, MON:
		return DriverEVM
	case FTM, ETC, EmROSE, AurETH, ACA, KLAY, OAS, CHZ, XDC, CHZ2:
		return DriverEVMLegacy
	case APTOS:
		return DriverAptos
	case ATOM, XPLA, INJ, HASH, LUNC, LUNA, SEI, TIA, NOBLE, AKT, BAND, ZETA, NIL, BABY, KAVA:
		return DriverCosmos
	case KAS:
		return DriverKaspa
	case EOS:
		return DriverEOS
	case SUI:
		return DriverSui
	case SOL:
		return DriverSolana
	case DOT, TAO, KSM, ENJ, KAR, ASTR:
		return DriverSubstrate
	case TRX:
		return DriverTron
	case TON:
		return DriverTon
	case XRP:
		return DriverXrp
	case XLM:
		return DriverXlm
	case FIL:
		return DriverFilecoin
	case DUSK:
		return DriverDusk
	case ADA:
		return DriverCardano
	}
	return ""
}

// Returns the signature algorithms supported by the driver
// The first algorithm will be used as the default.
func (driver Driver) SignatureAlgorithms() []SignatureType {
	switch driver {
	case DriverBitcoin:
		return []SignatureType{K256Sha256, Schnorr}
	case DriverBitcoinCash, DriverBitcoinLegacy, DriverCosmos, DriverXrp, DriverFilecoin, DriverEOS:
		return []SignatureType{K256Sha256}
	case DriverEVM, DriverEVMLegacy, DriverCosmosEvmos, DriverTron:
		return []SignatureType{K256Keccak}
	case DriverAptos, DriverSolana, DriverSui, DriverTon, DriverSubstrate, DriverXlm, DriverCardano:
		return []SignatureType{Ed255}
	case DriverDusk:
		return []SignatureType{Bls12_381G2Blake2}
	case DriverKaspa:
		return []SignatureType{Schnorr}
	}
	return []SignatureType{}
}

type PublicKeyFormat string

var Raw PublicKeyFormat = "raw"
var Compressed PublicKeyFormat = "compressed"
var Uncompressed PublicKeyFormat = "uncompressed"

func (driver Driver) PublicKeyFormat() PublicKeyFormat {
	switch driver {
	case DriverBitcoin, DriverCardano, DriverBitcoinCash, DriverBitcoinLegacy, DriverEOS:
		return Compressed
	case DriverCosmos, DriverCosmosEvmos, DriverXrp, DriverXlm:
		return Compressed
	case DriverEVM, DriverEVMLegacy, DriverTron, DriverFilecoin:
		return Uncompressed
	case DriverAptos, DriverSolana, DriverSui, DriverTon, DriverSubstrate, DriverDusk, DriverKaspa:
		return Raw
	}
	return ""
}

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

// External ID's used by other vendors for the given chain
type External struct {
	Dti string `yaml:"dti,omitempty"`

	CoinMarketCap struct {
		// CoinMarketCap's ID for the chain
		ChainId string `yaml:"chain_id,omitempty"`
		// CoinMarketCap's ID for the chain's native asset, also called "UCID"
		AssetId string `yaml:"asset_id,omitempty"`
	} `yaml:"coin_market_cap,omitempty"`
	CoinGecko struct {
		// TODO: is there a chain ID for coingecko?
		ChainId string `yaml:"chain_id,omitempty"`
		// Coingecko's asset ID, if relevant
		AssetId string `yaml:"asset_id,omitempty"`
	} `yaml:"coin_gecko,omitempty"`
	IndexingCo struct {
		ChainId  string `yaml:"chain_id,omitempty"`
		Disabled bool   `yaml:"disabled,omitempty"`
	} `yaml:"indexing_co,omitempty"`
}

type StakingConfig struct {
	// the contract used for staking, if relevant
	StakeContract string `yaml:"stake_contract,omitempty"`
	// the contract used for unstaking, if relevant
	UnstakeContract string `yaml:"unstake_contract,omitempty"`
	// Compatible providers for staking
	Providers []StakingProvider `yaml:"providers,omitempty"`
}

type AdditionalNativeAsset struct {
	// The asset ID of the asset to use
	AssetId string `yaml:"asset_id,omitempty"`
	// The on-chain contract ID of the asset
	ContractId ContractAddress `yaml:"contract_id,omitempty"`
	// Decimals for the asset
	Decimals int32 `yaml:"decimals,omitempty"`
	// Maximum fee limit
	FeeLimit AmountHumanReadable `yaml:"fee_limit"`
	Aliases  []string            `yaml:"aliases,omitempty"`
}

func NewAdditionalNativeAsset(assetId string, contractId ContractAddress, decimals int32, feeLimit AmountHumanReadable, aliases ...string) *AdditionalNativeAsset {
	return &AdditionalNativeAsset{
		assetId,
		contractId,
		decimals,
		feeLimit,
		aliases,
	}
}

func (na *AdditionalNativeAsset) HasAlias(alias string) bool {
	return slices.Contains(na.Aliases, alias)
}

type CrosschainClientConfig struct {
	Url     string          `yaml:"url"`
	Network NetworkSelector `yaml:"network,omitempty"`
}

func (staking *StakingConfig) Enabled() bool {
	return len(staking.Providers) > 0
}

func NewChainConfig(nativeAsset NativeAsset, driverMaybe ...Driver) *ChainConfig {
	driver := nativeAsset.Driver()
	if len(driverMaybe) > 0 {
		driver = driverMaybe[0]
	}
	cfg := &ChainConfig{
		ChainBaseConfig: &ChainBaseConfig{
			Chain:  nativeAsset,
			Driver: driver,
		},
		ChainClientConfig: &ChainClientConfig{},
	}
	cfg.Configure()
	return cfg
}
func (chain *ChainConfig) Base() *ChainBaseConfig {
	return chain.ChainBaseConfig
}
func (chain *ChainConfig) Client() *ChainClientConfig {
	return chain.ChainClientConfig
}

func (chain *ChainConfig) WithDriver(driver Driver) *ChainConfig {
	chain.Driver = driver
	return chain
}
func (chain *ChainConfig) WithDecimals(decimals int32) *ChainConfig {
	chain.Decimals = decimals
	return chain
}
func (chain *ChainConfig) WithUrl(url string) *ChainConfig {
	chain.ChainClientConfig.URL = url
	return chain
}
func (chain *ChainConfig) WithNet(net string) *ChainConfig {
	chain.ChainBaseConfig.Network = net
	return chain
}
func (chain *ChainConfig) WithChainCoin(chainCoin string) *ChainConfig {
	chain.ChainBaseConfig.ChainCoin = chainCoin
	return chain
}
func (chain *ChainConfig) WithChainPrefix(chainPrefix string) *ChainConfig {
	chain.ChainBaseConfig.ChainPrefix = StringOrInt(chainPrefix)
	return chain
}

func (chain *ChainConfig) WithProvider(provider string) *ChainConfig {
	chain.ChainClientConfig.Provider = provider
	return chain
}

func (chain *ChainConfig) WithMinGasPrice(minGasPrice float64) *ChainConfig {
	chain.ChainClientConfig.ChainMinGasPrice = minGasPrice
	return chain
}
func (chain *ChainConfig) WithMaxGasPrice(maxGasPrice float64) *ChainConfig {
	chain.ChainClientConfig.ChainMaxGasPrice = maxGasPrice
	return chain
}
func (chain *ChainConfig) WithFeeLimit(feeLimit AmountHumanReadable) *ChainConfig {
	chain.FeeLimit = feeLimit
	return chain
}
func (chain *ChainConfig) WithGasPriceMultiplier(multiplier float64) *ChainConfig {
	chain.ChainClientConfig.ChainGasMultiplier = multiplier
	return chain
}
func (chain *ChainConfig) WithGasBudgetDefault(gasBudgetDefault AmountHumanReadable) *ChainConfig {
	chain.ChainClientConfig.GasBudgetDefault = gasBudgetDefault
	return chain
}

func (chain *ChainConfig) WithAuth(auth config.Secret) *ChainConfig {
	chain.ChainClientConfig.Auth2 = auth
	return chain
}

func (chain *ChainConfig) WithChainID(chainID string) *ChainConfig {
	chain.ChainID = StringOrInt(chainID)
	return chain
}

func (chain *ChainConfig) WithIndexer(indexerType string, url string) *ChainConfig {
	chain.IndexerType = indexerType
	chain.IndexerUrl = url
	return chain
}

func (chain *ChainConfig) WithTransactionActiveTime(transactionActiveTime time.Duration) *ChainConfig {
	chain.TransactionActiveTime = transactionActiveTime
	return chain
}

type ChainConfig struct {
	*ChainBaseConfig   `yaml:",inline"`
	*ChainClientConfig `yaml:",inline"`
}

type ChainBaseConfig struct {
	// The crosschain symbol of the chain
	Chain NativeAsset `yaml:"chain,omitempty"`
	// The driver to use for the chain
	Driver Driver `yaml:"driver,omitempty"`
	// The network selector, if necessary (e.g. select mainnet, testnet, or devnet for bitcoin chains)
	Network string `yaml:"net,omitempty"`
	// Decimals for the chain's native asset (if it has one).
	Decimals int32 `yaml:"decimals,omitempty"`

	// The ChainID of the chain, either in integer or string format
	ChainID StringOrInt `yaml:"chain_id,omitempty"`

	// Human readable name of the chain, e.g. "Bitcoin"
	ChainName string `yaml:"chain_name,omitempty"`

	// Does the chain use a special prefix for it's address?
	// E.g. most cosmos chains do this.
	ChainPrefix StringOrInt `yaml:"chain_prefix,omitempty"`

	// If the chain has a native asset, and it has an actual contract address, it should be set here.
	// This is also referred to as the "ContractID".
	// E.g.
	// - APTOS has 0x1::aptos_coin::AptosCoin
	// - INJ has inj
	// - HASH has nhash
	// - LUNA has uluna
	ChainCoin string `yaml:"chain_coin,omitempty"`
	// Additional native assets that may be used to pay fees on the chain.
	NativeAssets []*AdditionalNativeAsset `yaml:"native_assets,omitempty"`
	// If true, then the `.Chain` does not represent any native asset (i.e. no chain-coin, no decimals).
	NoNativeAsset bool `yaml:"no_native_asset,omitempty"`

	// If necessary, specific which asset to use to spend for gas.
	GasCoin string `yaml:"gas_coin,omitempty"`

	// Indicate if this chain should not be included.
	Disabled *bool `yaml:"disabled,omitempty"`

	// Staking configuration
	Staking StakingConfig `yaml:"staking,omitempty"`

	// Maximum total fee limit: required for caller to make use of with `TxInput.GetFeeLimit()`
	FeeLimit AmountHumanReadable `yaml:"fee_limit,omitempty"`

	// Transfer tax is percentage that the network takes from every transfer .. only used so far for Terra Classic
	ChainTransferTax float64 `yaml:"chain_transfer_tax,omitempty"`

	// Used only for deriving private keys from mnemonic phrases in local testing
	ChainCoinHDPath uint32 `yaml:"chain_coin_hd_path,omitempty"`

	// Should use `ChainID` instead
	XChainIDStr string `yaml:"chain_id_str,omitempty"`
}

func (chain *ChainConfig) Configure() {
	chain.ChainClientConfig.Configure()
	if chain.XChainIDStr != "" {
		logrus.Warnf("chain_id_str is deprecated, use chain_id instead")
		chain.ChainID = StringOrInt(chain.XChainIDStr)
	}
}

type ChainClientConfig struct {
	////////////////////////////////
	///// RPC / CLIENT CONFIGURATION
	////////////////////////////////

	URL          string `yaml:"url,omitempty"`
	SecondaryURL string `yaml:"secondary_url,omitempty"`

	// Set a secret reference, see config/secret.go.  Used for setting an API keys.
	Auth2 config.Secret `yaml:"auth,omitempty"`

	// Optional configuration of the Driver.  Some chains support different kinds of RPC.
	Provider         string                 `yaml:"provider,omitempty"`
	CrosschainClient CrosschainClientConfig `yaml:"crosschain_client"`

	// Does the chain rely on an indexer in addition to RPC?  If so, the URL and type
	// may be set here.
	IndexerUrl  string `yaml:"indexer_url,omitempty"`
	IndexerType string `yaml:"indexer_type,omitempty"`
	// Maximun depth to scan for transaction, if there is no index to use (substrate...)
	MaxScanDepth int `yaml:"max_scan_depth,omitempty"`

	NoGasFees bool `yaml:"no_gas_fees,omitempty"`

	// Default gas budget to use for client gas estimation
	GasBudgetDefault AmountHumanReadable `yaml:"gas_budget_default,omitempty"`
	// Gas budget that cannot be exceeded below
	GasBudgetMinimum AmountHumanReadable `yaml:"gas_budget_min,omitempty"`
	// A remainder balance (e.g. rent threshold) that must be maintained after a transfer.
	ReserveAmount AmountHumanReadable `yaml:"reserve_amount,omitempty"`
	// A default for clients to gas price if there's not better way to estimate.
	ChainGasPriceDefault float64 `yaml:"chain_gas_price_default,omitempty"`
	// A local multiplier for client to apply to gas estimation, if it's important/needed.
	ChainGasMultiplier          float64 `yaml:"chain_gas_multiplier,omitempty"`
	SecondaryChainGasMultiplier float64 `yaml:"secondary_chain_gas_multiplier,omitempty"`
	// for gas estimation of gas limit, for somechains the simulation may be flaky and need a multiplier
	ChainGasLimitMultiplier float64 `yaml:"chain_gas_limit_multiplier,omitempty"`
	// The multiplier to apply to gas when there is a minimum replacement increase.
	// This is used for EVM chains, to avoid the "replacement transaction underpriced" message.
	// Normally it is 10%, but on chains like Base and Optimism, it seems to be at least 25% in practice.
	ReplacementTransactionMultiplier float64 `yaml:"replacement_transaction_multiplier,omitempty"`
	// The max/min prices can be set to provide sanity limits for what a gas price (per gas or per byte) should be.
	// This should be in the blockchain amount.
	ChainMaxGasPrice float64 `yaml:"chain_max_gas_price,omitempty"`
	ChainMinGasPrice float64 `yaml:"chain_min_gas_price,omitempty"`
	// Default gas limit for transactions
	GasLimitDefault int `yaml:"gas_limit_default,omitempty"`
	// TransactionActiveTime specifies the duration for which a transaction remains valid after being submitted.
	// The value is represented as a `time.Duration` string.
	// This field is currently used only by the Stellar network.
	//
	// Example format: "30s" (30 seconds), "2m" (2 minutes), "1h" (1 hour).
	TransactionActiveTime time.Duration `yaml:"transaction_active_time,omitempty"`
	// How many confirmations is considered "final" for this chain?
	ConfirmationsFinal int `yaml:"confirmations_final,omitempty"`

	// Gas price oracle address
	// Currently this is used for EVM L2 chains that have an additional "l1" fee.
	GasPriceOracleAddress string `yaml:"gas_price_oracle_address,omitempty"`

	// Rate limit setting on RPC requests for client, in requests/second.
	RateLimit rate.Limit `yaml:"rate_limit,omitempty"`
	// Period between requests (alternative to `rate_limit`)
	PeriodLimit time.Duration `yaml:"period_limit,omitempty"`
	// Number of requests to permit in burst
	Burst int `yaml:"burst,omitempty"`

	// Rate limiter configured from `rate_limit`, `period_limit`, `burst` (requires calling .Configure after loading from config)
	Limiter *rate.Limiter `yaml:"-" mapstructure:"-"`

	// Additional metadata.  Not Used in crosschain itself, but helpful to enrich API endpoints.
	External External `yaml:"external,omitempty"`
	// Informational URLs for the chain explorers.
	ExplorerUrls ExplorerUrls `yaml:"explorer_urls,omitempty"`
	// If true, this means that the chain is not intended for future use and
	// is only maintained for legacy purposes (by the upstream community).
	// E.g. "Terra Classic" or "Fantom" are legacy chains that each have replacements.
	Legacy bool `yaml:"legacy,omitempty"`
}

func (chain *ChainClientConfig) NewClientLimiter() *rate.Limiter {
	// default no limit
	burst := chain.Burst
	var limiter = rate.NewLimiter(rate.Inf, burst)
	if chain.PeriodLimit != 0 {
		limiter = rate.NewLimiter(rate.Every(chain.PeriodLimit), burst)
	}
	if chain.RateLimit != 0 {
		limiter = rate.NewLimiter(chain.RateLimit, burst)
	}
	return limiter
}

func (chain *ChainClientConfig) Configure() {
	chain.Limiter = chain.NewClientLimiter()
}

var _ ITask = &ChainConfig{}

func (c ChainConfig) String() string {
	secretRef := string(c.Auth2)
	if !config.HasTypePrefix(secretRef) || strings.HasPrefix(secretRef, string(config.Raw)) {
		secretRef = "<REDACTED>"
	}

	return fmt.Sprintf(
		"NativeAssetConfig(asset=%s chainId=%s driver=%s chainCoin=%s prefix=%s net=%s url=%s auth=%s provider=%s)",
		c.Chain, c.ChainID, c.Driver, c.ChainCoin, c.ChainPrefix, c.Network, c.URL, secretRef, c.Provider,
	)
}

func (asset *ChainConfig) GetDecimals() int32 {
	return asset.Decimals
}

func (asset *ChainConfig) GetChain() *ChainConfig {
	return asset
}

func (native *ChainConfig) GetContract() string {
	return ""
}

func (native *ChainConfig) GetAssetSymbol() string {
	return string(native.Chain)
}

// Returns URL and driver used for the client.  This will either
// Be the chain driver, or the 'special' crosschain driver.
func (native *ChainConfig) ClientURL() (string, Driver) {
	if native.URL == "" || native.URL == "-" {
		if native.CrosschainClient.Url != "" {
			return native.CrosschainClient.Url, DriverCrosschain
		}
		return "https://connector.cordialapis.com", DriverCrosschain
	}
	return native.URL, native.Driver
}

func (native *ChainConfig) IsChain(contract ContractAddress) bool {
	return contract == "" || native.Chain == NativeAsset(contract)
}
