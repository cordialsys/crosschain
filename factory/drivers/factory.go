package drivers

import (
	"errors"
	"fmt"

	. "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/aptos"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	bitcoinaddress "github.com/cordialsys/crosschain/chain/bitcoin/address"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/cordialsys/crosschain/chain/cosmos"
	cosmosaddress "github.com/cordialsys/crosschain/chain/cosmos/address"
	cosmosbuilder "github.com/cordialsys/crosschain/chain/cosmos/builder"
	cosmosclient "github.com/cordialsys/crosschain/chain/cosmos/client"
	"github.com/cordialsys/crosschain/chain/evm"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	evmbuilder "github.com/cordialsys/crosschain/chain/evm/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/client/staking/figment"
	"github.com/cordialsys/crosschain/chain/evm/client/staking/kiln"
	evm_legacy "github.com/cordialsys/crosschain/chain/evm_legacy"
	"github.com/cordialsys/crosschain/chain/solana"
	solanaaddress "github.com/cordialsys/crosschain/chain/solana/address"
	solanabuilder "github.com/cordialsys/crosschain/chain/solana/builder"
	solanaclient "github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/ton"
	tonaddress "github.com/cordialsys/crosschain/chain/ton/address"
	"github.com/cordialsys/crosschain/chain/tron"
	xrpaddress "github.com/cordialsys/crosschain/chain/xrp/address"
	xrpclient "github.com/cordialsys/crosschain/chain/xrp/client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/services"
	"github.com/cordialsys/crosschain/factory/signer"
)

func NewClient(cfg ITask, driver Driver) (xclient.FullClient, error) {
	switch driver {
	case DriverEVM:
		return evmclient.NewClient(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewClient(cfg)
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
		return substrate.NewClient(cfg)
	case DriverTron:
		return tron.NewClient(cfg)
	case DriverTon:
		return ton.NewClient(cfg)
	case DriverXrp:
		return xrpclient.NewClient(cfg)
	}
	return nil, errors.New("no client defined for chains: " + string(cfg.ID()))
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
	}
	return nil, fmt.Errorf("no staking client defined for %s on %s", provider, driver)
}

func NewTxBuilder(cfg ITask) (xcbuilder.FullTransferBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
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
		return bitcoin.NewTxBuilder(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewTxBuilder(cfg)
	case DriverSubstrate:
		return substrate.NewTxBuilder(cfg)
	case DriverTron:
		return tron.NewTxBuilder(cfg)
	case DriverTon:
		return ton.NewTxBuilder(cfg)
	}
	return nil, errors.New("no tx-builder defined for: " + string(cfg.ID()))
}

func NewSigner(cfg ITask, secret string) (*signer.Signer, error) {
	chain := cfg.GetChain()
	return signer.New(chain.Driver, secret, chain)
}

func NewAddressBuilder(cfg ITask) (AddressBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM:
		return evmaddress.NewAddressBuilder(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewAddressBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmosaddress.NewAddressBuilder(cfg)
	case DriverSolana:
		return solanaaddress.NewAddressBuilder(cfg)
	case DriverAptos:
		return aptos.NewAddressBuilder(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoinaddress.NewAddressBuilder(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewAddressBuilder(cfg)
	case DriverSui:
		return sui.NewAddressBuilder(cfg)
	case DriverSubstrate:
		return substrate.NewAddressBuilder(cfg)
	case DriverTron:
		return tron.NewAddressBuilder(cfg)
	case DriverTon:
		return tonaddress.NewAddressBuilder(cfg)
	case DriverXrp:
		return xrpaddress.NewAddressBuilder(cfg)
	}
	return nil, errors.New("no address builder defined for: " + string(cfg.ID()))
}

func CheckError(driver Driver, err error) xclient.ClientError {
	switch driver {
	case DriverEVM:
		return evm.CheckError(err)
	case DriverEVMLegacy:
		return evm.CheckError(err)
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
		//case DriverXrp:
		//	return xrp.CheckError(err)
	}
	return xclient.UnknownError
}
