package drivers

import (
	"fmt"

	"github.com/cordialsys/crosschain/chain/eos"
	"github.com/cordialsys/crosschain/chain/kaspa"
	kaspaaddress "github.com/cordialsys/crosschain/chain/kaspa/address"
	kaspabuilder "github.com/cordialsys/crosschain/chain/kaspa/builder"
	kaspaclient "github.com/cordialsys/crosschain/chain/kaspa/client"
	"github.com/cordialsys/crosschain/chain/substrate"
	xrpbuilder "github.com/cordialsys/crosschain/chain/xrp/builder"

	. "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	bitcoinaddress "github.com/cordialsys/crosschain/chain/bitcoin/address"
	bitcoinbuilder "github.com/cordialsys/crosschain/chain/bitcoin/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	cardano "github.com/cordialsys/crosschain/chain/cardano"
	cardanoaddress "github.com/cordialsys/crosschain/chain/cardano/address"
	cardanobuilder "github.com/cordialsys/crosschain/chain/cardano/builder"
	cardanoclient "github.com/cordialsys/crosschain/chain/cardano/client"
	"github.com/cordialsys/crosschain/chain/cosmos"
	cosmosaddress "github.com/cordialsys/crosschain/chain/cosmos/address"
	cosmosbuilder "github.com/cordialsys/crosschain/chain/cosmos/builder"
	cosmosclient "github.com/cordialsys/crosschain/chain/cosmos/client"
	dusk "github.com/cordialsys/crosschain/chain/dusk"
	duskaddress "github.com/cordialsys/crosschain/chain/dusk/address"
	duskbuilder "github.com/cordialsys/crosschain/chain/dusk/builder"
	duskclient "github.com/cordialsys/crosschain/chain/dusk/client"
	eosaddress "github.com/cordialsys/crosschain/chain/eos/address"
	eosbuilder "github.com/cordialsys/crosschain/chain/eos/builder"
	eosclient "github.com/cordialsys/crosschain/chain/eos/client"
	"github.com/cordialsys/crosschain/chain/evm"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/client/staking/figment"
	"github.com/cordialsys/crosschain/chain/evm/client/staking/kiln"
	evm_legacy "github.com/cordialsys/crosschain/chain/evm_legacy"
	fil "github.com/cordialsys/crosschain/chain/filecoin"
	filaddress "github.com/cordialsys/crosschain/chain/filecoin/address"
	filbuilder "github.com/cordialsys/crosschain/chain/filecoin/builder"
	filclient "github.com/cordialsys/crosschain/chain/filecoin/client"
	"github.com/cordialsys/crosschain/chain/solana"
	solanaaddress "github.com/cordialsys/crosschain/chain/solana/address"
	solanabuilder "github.com/cordialsys/crosschain/chain/solana/builder"
	solanaclient "github.com/cordialsys/crosschain/chain/solana/client"
	substrateaddress "github.com/cordialsys/crosschain/chain/substrate/address"
	substratebuilder "github.com/cordialsys/crosschain/chain/substrate/builder"
	substrateclient "github.com/cordialsys/crosschain/chain/substrate/client"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/ton"
	tonaddress "github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/cordialsys/crosschain/chain/tron"
	xlm "github.com/cordialsys/crosschain/chain/xlm"
	xlmaddress "github.com/cordialsys/crosschain/chain/xlm/address"
	xlmbuilder "github.com/cordialsys/crosschain/chain/xlm/builder"
	xlmclient "github.com/cordialsys/crosschain/chain/xlm/client"
	xrp "github.com/cordialsys/crosschain/chain/xrp"
	xrpaddress "github.com/cordialsys/crosschain/chain/xrp/address"
	xrpclient "github.com/cordialsys/crosschain/chain/xrp/client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/factory/signer"
)

func NewClient(cfg ITask, driver Driver) (xclient.Client, error) {
	switch driver {
	case DriverCardano:
		return cardanoclient.NewClient(cfg)
	case DriverEVM:
		return evmclient.NewClient(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewClient(cfg)
	case DriverFilecoin:
		return filclient.NewClient(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmosclient.NewClient(cfg)
	case DriverSolana:
		return solanaclient.NewClient(cfg)
	case DriverAptos:
		return aptos.NewClient(cfg)
	case DriverSui:
		return sui.NewClient(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoin.NewClient(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewClient(cfg)
	case DriverSubstrate:
		return substrateclient.NewClient(cfg)
	case DriverTron:
		return tron.NewClient(cfg)
	case DriverTon:
		return ton.NewClient(cfg)
	case DriverXrp:
		return xrpclient.NewClient(cfg)
	case DriverXlm:
		return xlmclient.NewClient(cfg)
	case DriverDusk:
		return duskclient.NewClient(cfg)
	case DriverKaspa:
		return kaspaclient.NewClient(cfg)
	case DriverEOS:
		return eosclient.NewClient(cfg)
	}
	return nil, fmt.Errorf("no client defined for chain: %s", string(cfg.GetChain().Chain))
}

func NewStakingClient(servicesConfig *services.ServicesConfig, cfg ITask, provider StakingProvider) (xclient.StakingClient, error) {
	driver := cfg.GetChain().Driver
	switch driver {
	case DriverEVM:
		switch provider {
		case Kiln:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return kiln.NewClient(rpcClient, cfg.GetChain(), &servicesConfig.Kiln)
		case Figment:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return figment.NewClient(rpcClient, cfg.GetChain(), &servicesConfig.Figment)
		case Twinstake:
			return nil, fmt.Errorf("not implemented")
		case Native:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return rpcClient, nil
		}
	case DriverCosmos, DriverCosmosEvmos:
		return cosmosclient.NewClient(cfg)
	case DriverSolana:
		return solanaclient.NewClient(cfg)
	case DriverSubstrate:
		return substrateclient.NewClient(cfg)
	case DriverEOS:
		return eosclient.NewClient(cfg)
	}
	return nil, fmt.Errorf("no staking client defined for %s on %s", provider, driver)
}

func NewTxBuilder(cfg *ChainBaseConfig) (xcbuilder.FullTransferBuilder, error) {
	switch Driver(cfg.Driver) {
	case DriverCardano:
		return cardanobuilder.NewTxBuilder(cfg)
	case DriverEVM:
		return evmbuilder.NewTxBuilder(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewTxBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmosbuilder.NewTxBuilder(cfg)
	case DriverSolana:
		return solanabuilder.NewTxBuilder(cfg)
	case DriverAptos:
		return aptos.NewTxBuilder(cfg)
	case DriverSui:
		return sui.NewTxBuilder(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoinbuilder.NewTxBuilder(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewTxBuilder(cfg)
	case DriverSubstrate:
		return substratebuilder.NewTxBuilder(cfg)
	case DriverTron:
		return tron.NewTxBuilder(cfg)
	case DriverTon:
		return ton.NewTxBuilder(cfg)
	case DriverXrp:
		return xrpbuilder.NewTxBuilder(cfg)
	case DriverXlm:
		return xlmbuilder.NewTxBuilder(cfg)
	case DriverFilecoin:
		return filbuilder.NewTxBuilder(cfg)
	case DriverDusk:
		return duskbuilder.NewTxBuilder(cfg)
	case DriverKaspa:
		return kaspabuilder.NewTxBuilder(cfg)
	case DriverEOS:
		return eosbuilder.NewTxBuilder(cfg)
	}
	return nil, fmt.Errorf("no tx-builder defined for: %s", string(cfg.Chain))
}

func NewSigner(chain *ChainBaseConfig, secret string, options ...xcaddress.AddressOption) (*signer.Signer, error) {
	return signer.New(chain.Driver, secret, chain, options...)
}

func NewAddressBuilder(cfg *ChainBaseConfig, options ...xcaddress.AddressOption) (AddressBuilder, error) {
	switch Driver(cfg.Driver) {
	case DriverDusk:
		return duskaddress.NewAddressBuilder(cfg)
	case DriverEVM:
		return evmaddress.NewAddressBuilder(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewAddressBuilder(cfg)
	case DriverFilecoin:
		return filaddress.NewAddressBuilder(cfg, options...)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmosaddress.NewAddressBuilder(cfg)
	case DriverSolana:
		return solanaaddress.NewAddressBuilder(cfg)
	case DriverAptos:
		return aptos.NewAddressBuilder(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoinaddress.NewAddressBuilder(cfg, options...)
	case DriverBitcoinCash:
		return bitcoin_cash.NewAddressBuilder(cfg)
	case DriverSui:
		return sui.NewAddressBuilder(cfg)
	case DriverSubstrate:
		return substrateaddress.NewAddressBuilder(cfg)
	case DriverTron:
		return tron.NewAddressBuilder(cfg)
	case DriverTon:
		return tonaddress.NewAddressBuilder(cfg)
	case DriverXrp:
		return xrpaddress.NewAddressBuilder(cfg)
	case DriverXlm:
		return xlmaddress.NewAddressBuilder(cfg)
	case DriverCardano:
		return cardanoaddress.NewAddressBuilder(cfg)
	case DriverKaspa:
		return kaspaaddress.NewAddressBuilder(cfg)
	case DriverEOS:
		return eosaddress.NewAddressBuilder(cfg)
	}
	return nil, fmt.Errorf("no address builder defined for: %s", string(cfg.Chain))
}

func CheckError(driver Driver, err error) errors.Status {
	if err, ok := err.(*errors.Error); ok {
		return err.Status
	}
	switch driver {
	case DriverCardano:
		return cardano.CheckError(err)
	case DriverDusk:
		return dusk.CheckError(err)
	case DriverEVM:
		return evm.CheckError(err)
	case DriverEVMLegacy:
		return evm.CheckError(err)
	case DriverFilecoin:
		return fil.CheckError(err)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.CheckError(err)
	case DriverSolana:
		return solana.CheckError(err)
	case DriverAptos:
		return aptos.CheckError(err)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoin.CheckError(err)
	case DriverBitcoinCash:
		return bitcoin_cash.CheckError(err)
	case DriverSui:
		return sui.CheckError(err)
	case DriverSubstrate:
		return substrate.CheckError(err)
	case DriverTron:
		return tron.CheckError(err)
	case DriverTon:
		return ton.CheckError(err)
	case DriverXrp:
		return xrp.CheckError(err)
	case DriverXlm:
		return xlm.CheckError(err)
	case DriverKaspa:
		return kaspa.CheckError(err)
	case DriverEOS:
		return eos.CheckError(err)
	}
	return errors.UnknownError
}
