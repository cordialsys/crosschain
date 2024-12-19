package types

import (
	authzmodule "cosmossdk.io/x/authz/module"
	"cosmossdk.io/x/bank"
	distr "cosmossdk.io/x/distribution"
	"cosmossdk.io/x/evidence"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/mint"
	"cosmossdk.io/x/params"
	"cosmossdk.io/x/slashing"
	"cosmossdk.io/x/staking"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/upgrade"
	xc "github.com/cordialsys/crosschain"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	cosmTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"

	"github.com/cosmos/cosmos-sdk/x/genutil"
)

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
var ModuleBasics = module.NewManager(
	auth.AppModule{},
	genutil.AppModule{},
	bank.AppModule{},
	staking.AppModule{},
	mint.AppModule{},
	distr.AppModule{},
	params.AppModule{},
	slashing.AppModule{},
	feegrantmodule.AppModule{},
	upgrade.AppModule{},
	evidence.AppModule{},
	authzmodule.AppModule{},
)

func NewEncodingConfig(chainCfg *xc.ChainConfig) (EncodingConfig, error) {
	amino := codec.NewLegacyAmino()

	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          address.NewBech32Codec(chainCfg.ChainPrefix),
			ValidatorAddressCodec: address.NewBech32Codec(chainCfg.ChainPrefix + "valoper"),
		},
	})
	if err != nil {
		return EncodingConfig{}, err
	}
	signingContext := interfaceRegistry.SigningContext()

	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := cosmTx.NewTxConfig(
		marshaler,
		signingContext.AddressCodec(),
		signingContext.ValidatorAddressCodec(),
		cosmTx.DefaultSignModes,
	)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		Amino:             amino,
		TxConfig:          txCfg,
	}, nil
}

var legacyCodecRegistered = false

// MakeEncodingConfig creates an EncodingConfig for testing
func MakeEncodingConfig(chainCfg *xc.ChainConfig) (EncodingConfig, error) {
	encodingConfig, err := NewEncodingConfig(chainCfg)
	if err != nil {
		return encodingConfig, err
	}
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

	return encodingConfig, nil
}

func MakeCosmosConfig(chainCfg *xc.ChainConfig) (EncodingConfig, error) {
	cosmosCfg, err := MakeEncodingConfig(chainCfg)
	if err != nil {
		return cosmosCfg, err
	}
	// Register types from other chains that use potentially incompatible cosmos-sdk versions
	RegisterExternalInterfaces(cosmosCfg.InterfaceRegistry)
	RegisterExternalLegacyAdmino(cosmosCfg.Amino)
	return cosmosCfg, nil
}
