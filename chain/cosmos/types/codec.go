package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	wasmd "github.com/jumpcrypto/crosschain/chain/cosmos/types/CosmWasm/wasmd/x/wasm/types"
	injethsecp256k1 "github.com/jumpcrypto/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	injective "github.com/jumpcrypto/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/types"
	terraclassic "github.com/jumpcrypto/crosschain/chain/cosmos/types/classic-terra/core/v2/x/vesting/types"
	"github.com/jumpcrypto/crosschain/chain/cosmos/types/evmos/ethermint/crypto/ethsecp256k1"
	etherminttypes "github.com/jumpcrypto/crosschain/chain/cosmos/types/evmos/ethermint/types"
	ethermintevm "github.com/jumpcrypto/crosschain/chain/cosmos/types/evmos/ethermint/x/evm/types"
	ethermintfeemarket "github.com/jumpcrypto/crosschain/chain/cosmos/types/evmos/ethermint/x/feemarket/types"
)

// Register types from other chains.  Do not rely on 3rd party dependencies here!
// Other cosmos chains will rely on differing (and incompatible) versions of cosmos-sdk,
// which causes a lot of pain to maintain this library.
// Instead, copy the relevant protobuf to proto/ and compile it.  Then copy the relevant type registrations
// from the target chain's x/module/types/codec.go to this file.
func RegisterExternalInterfaces(registry codectypes.InterfaceRegistry) {
	registerInterfacesTerraClassic(registry)
	registerInterfacesEthermint(registry)
	registerInterfacesCosmosExtra(registry)
	registerInterfacesInjective(registry)
	registerInterfacesWasmd(registry)
}
func RegisterExternalLegacyAdmino(cdc *codec.LegacyAmino) {
	registerLegacyAminoTerraClassic(cdc)
}

func registerInterfacesInjective(registry codectypes.InterfaceRegistry) {
	registry.RegisterInterface("injective.types.v1beta1.EthAccount", (*authtypes.AccountI)(nil))

	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&injective.EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&injective.EthAccount{},
	)
	registry.RegisterInterface("injective.types.v1beta1.ExtensionOptionsWeb3Tx", (*injective.ExtensionOptionsWeb3TxI)(nil))
	registry.RegisterImplementations(
		(*injective.ExtensionOptionsWeb3TxI)(nil),
		&injective.ExtensionOptionsWeb3Tx{},
	)
	registry.RegisterInterface("injective.types.v1beta1.ExtensionOptionI", (*injective.ExtensionOptionI)(nil))
	registry.RegisterImplementations(
		(*injective.ExtensionOptionI)(nil),
		&injective.ExtensionOptionsWeb3Tx{},
	)
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &injethsecp256k1.PubKey{})
}

func registerInterfacesCosmosExtra(registry codectypes.InterfaceRegistry) {
	// cosmos vesting types not registered by default it seems
	vestingtypes.RegisterInterfaces(registry)
}
func registerInterfacesEthermint(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&etherminttypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&etherminttypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&ethermintevm.ExtensionOptionsEthereumTx{},
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&ethermintevm.MsgEthereumTx{},
		&ethermintevm.MsgUpdateParams{},
	)
	registry.RegisterInterface(
		"ethermint.evm.v1.TxData",
		(*ethermintevm.TxData)(nil),
		&ethermintevm.DynamicFeeTx{},
		&ethermintevm.AccessListTx{},
		&ethermintevm.LegacyTx{},
	)

	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&ethermintfeemarket.MsgUpdateParams{},
	)

	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &ethsecp256k1.PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &ethsecp256k1.PrivKey{})
}

func registerInterfacesTerraClassic(registry codectypes.InterfaceRegistry) {
	registry.RegisterInterface(
		"cosmos.vesting.v1beta1.VestingAccount",
		(*exported.VestingAccount)(nil),
		&terraclassic.LazyGradedVestingAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&vestingtypes.BaseVestingAccount{},
		&terraclassic.LazyGradedVestingAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		&vestingtypes.BaseVestingAccount{},
		&terraclassic.LazyGradedVestingAccount{},
	)

	// Do we also need to include these?
	// msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

func registerLegacyAminoTerraClassic(cdc *codec.LegacyAmino) {
	cdc.RegisterInterface((*exported.VestingAccount)(nil), nil)
	cdc.RegisterConcrete(&vestingtypes.BaseVestingAccount{}, "core/BaseVestingAccount", nil)
	cdc.RegisterConcrete(&terraclassic.LazyGradedVestingAccount{}, "core/LazyGradedVestingAccount", nil)
}

func registerInterfacesWasmd(registry codectypes.InterfaceRegistry) {
	registry.RegisterInterface("injective.types.v1beta1.EthAccount", (*authtypes.AccountI)(nil))

	registry.RegisterImplementations(
		(*authtypes.AccountI)(nil),
		&injective.EthAccount{},
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&wasmd.MsgStoreCode{},
		&wasmd.MsgInstantiateContract{},
		&wasmd.MsgInstantiateContract2{},
		&wasmd.MsgExecuteContract{},
		&wasmd.MsgMigrateContract{},
		&wasmd.MsgUpdateAdmin{},
		&wasmd.MsgClearAdmin{},
		// &wasmd.MsgIBCCloseChannel{},
		// &wasmd.MsgIBCSend{},
		&wasmd.MsgUpdateInstantiateConfig{},
		&wasmd.MsgUpdateParams{},
		&wasmd.MsgSudoContract{},
		&wasmd.MsgPinCodes{},
		&wasmd.MsgUnpinCodes{},
		&wasmd.MsgStoreAndInstantiateContract{},
	)
	// registry.RegisterImplementations(
	// 	(*v1beta1.Content)(nil),
	// 	&wasmd.StoreCodeProposal{},
	// 	&wasmd.InstantiateContractProposal{},
	// 	&wasmd.InstantiateContract2Proposal{},
	// 	&wasmd.MigrateContractProposal{},
	// 	&wasmd.SudoContractProposal{},
	// 	&wasmd.ExecuteContractProposal{},
	// 	&wasmd.UpdateAdminProposal{},
	// 	&wasmd.ClearAdminProposal{},
	// 	&wasmd.PinCodesProposal{},
	// 	&wasmd.UnpinCodesProposal{},
	// 	&wasmd.UpdateInstantiateConfigProposal{},
	// 	&wasmd.StoreAndInstantiateContractProposal{},
	// )

	// registry.RegisterInterface("cosmwasm.wasm.v1.ContractInfoExtension", (*wasmd.ContractInfoExtension)(nil))

	// registry.RegisterInterface("cosmwasm.wasm.v1.ContractAuthzFilterX", wasmd.(*ContractAuthzFilterX)(nil))
	// registry.RegisterImplementations(
	// 	(*wasmd.ContractAuthzFilterX)(nil),
	// 	&wasmd.AllowAllMessagesFilter{},
	// 	&wasmd.AcceptedMessageKeysFilter{},
	// 	&wasmd.AcceptedMessagesFilter{},
	// )

	// registry.RegisterInterface("cosmwasm.wasm.v1.ContractAuthzLimitX", (*wasmd.ContractAuthzLimitX)(nil))
	// registry.RegisterImplementations(
	// 	(*ContractAuthzLimitX)(nil),
	// 	&MaxCallsLimit{},
	// 	&MaxFundsLimit{},
	// 	&CombinedLimit{},
	// )

	// registry.RegisterImplementations(
	// 	(*authz.Authorization)(nil),
	// 	&ContractExecutionAuthorization{},
	// 	&ContractMigrationAuthorization{},
	// )
}
