package cosmos

import (
	"crypto/sha256"

	xc "github.com/jumpcrypto/crosschain"

	// "github.com/terra-money/core/x/vesting"

	"github.com/CosmWasm/wasmd/x/wasm"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	cosmTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	localcodectypes "github.com/jumpcrypto/crosschain/chain/cosmos/types"
	injethsecp256k1 "github.com/jumpcrypto/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	"github.com/jumpcrypto/crosschain/chain/cosmos/types/evmos/ethermint/crypto/ethsecp256k1"
)

const LEN_NATIVE_ASSET = 8

var legacyCodecRegistered = false

// EncodingConfig specifies the concrete encoding types to use for a given app.
// This is provided for compatibility between protobuf and amino implementations.
type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Marshaler         codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

// ModuleBasics defines the module BasicManager is in charge of setting up basic,
// non-dependant module elements, such as codec registration
// and genesis verification.
var ModuleBasics = module.NewBasicManager(
	auth.AppModuleBasic{},
	genutil.AppModuleBasic{},
	bank.AppModuleBasic{},
	capability.AppModuleBasic{},
	staking.AppModuleBasic{},
	mint.AppModuleBasic{},
	distr.AppModuleBasic{},
	params.AppModuleBasic{},
	crisis.AppModuleBasic{},
	slashing.AppModuleBasic{},
	// ibc.AppModuleBasic{},
	feegrantmodule.AppModuleBasic{},
	upgrade.AppModuleBasic{},
	evidence.AppModuleBasic{},
	// transfer.AppModuleBasic{},
	// vesting.AppModuleBasic{},
	// ica.AppModuleBasic{},
	// router.AppModuleBasic{},
	authzmodule.AppModuleBasic{},
	wasm.AppModuleBasic{},
)

func NewEncodingConfig() EncodingConfig {
	amino := codec.NewLegacyAmino()
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := cosmTx.NewTxConfig(marshaler, cosmTx.DefaultSignModes)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		Amino:             amino,
		TxConfig:          txCfg,
	}
}

// MakeEncodingConfig creates an EncodingConfig for testing
func MakeEncodingConfig() EncodingConfig {
	encodingConfig := NewEncodingConfig()
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	if !legacyCodecRegistered {
		// authz module use this codec to get signbytes.
		// authz MsgExec can execute all message types,
		// so legacy.Cdc need to register all amino messages to get proper signature
		ModuleBasics.RegisterLegacyAminoCodec(legacy.Cdc)
		legacyCodecRegistered = true
	}

	return encodingConfig
}

func MakeCosmosConfig() EncodingConfig {
	cosmosCfg := MakeEncodingConfig()
	// Register types from other chains that use potentially incompatible cosmos-sdk versions
	localcodectypes.RegisterExternalInterfaces(cosmosCfg.InterfaceRegistry)
	localcodectypes.RegisterExternalLegacyAdmino(cosmosCfg.Amino)
	return cosmosCfg
}

func isNativeAsset(asset *xc.AssetConfig) bool {
	return asset.Type == xc.AssetTypeNative || len(asset.Contract) < LEN_NATIVE_ASSET
}

func isEVMOS(asset xc.NativeAssetConfig) bool {
	return xc.Driver(asset.Driver) == xc.DriverCosmosEvmos
}

func isINJ(asset xc.NativeAssetConfig) bool {
	return asset.NativeAsset == xc.NativeAsset("INJ")
}

func getPublicKey(asset xc.NativeAssetConfig, publicKeyBytes xc.PublicKey) cryptotypes.PubKey {
	if isINJ(asset) {
		return &injethsecp256k1.PubKey{Key: publicKeyBytes}
	}
	if isEVMOS(asset) {
		return &ethsecp256k1.PubKey{Key: publicKeyBytes}
	}
	return &secp256k1.PubKey{Key: publicKeyBytes}
}

func getSighash(asset xc.NativeAssetConfig, sigData []byte) []byte {
	if isEVMOS(asset) || isINJ(asset) {
		return crypto.Keccak256(sigData)
	}
	sighash := sha256.Sum256(sigData)
	return sighash[:]
}
