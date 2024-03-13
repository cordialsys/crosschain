package drivers

import (
	"errors"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/cordialsys/crosschain/chain/cosmos"
	"github.com/cordialsys/crosschain/chain/evm"
	evm_legacy "github.com/cordialsys/crosschain/chain/evm_legacy"
	"github.com/cordialsys/crosschain/chain/solana"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/tron"
	xclient "github.com/cordialsys/crosschain/client"
)

func NewClient(cfg ITask, driver Driver) (xclient.Client, error) {
	switch driver {
	case DriverEVM:
		return evm.NewClient(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewClient(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewClient(cfg)
	case DriverSolana:
		return solana.NewClient(cfg)
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
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewTxBuilder(cfg ITask) (TxBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM:
		return evm.NewTxBuilder(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewTxBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewTxBuilder(cfg)
	case DriverSolana:
		return solana.NewTxBuilder(cfg)
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
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewSigner(cfg ITask) (Signer, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM:
		return evm.NewSigner(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewSigner(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewSigner(cfg)
	case DriverSolana:
		return solana.NewSigner(cfg)
	case DriverAptos:
		return aptos.NewSigner(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoin.NewSigner(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewSigner(cfg)
	case DriverSui:
		return sui.NewSigner(cfg)
	case DriverSubstrate:
		return substrate.NewSigner(cfg)
	case DriverTron:
		return tron.NewSigner(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewAddressBuilder(cfg ITask) (AddressBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM:
		return evm.NewAddressBuilder(cfg)
	case DriverEVMLegacy:
		return evm_legacy.NewAddressBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewAddressBuilder(cfg)
	case DriverSolana:
		return solana.NewAddressBuilder(cfg)
	case DriverAptos:
		return aptos.NewAddressBuilder(cfg)
	case DriverBitcoin, DriverBitcoinLegacy:
		return bitcoin.NewAddressBuilder(cfg)
	case DriverBitcoinCash:
		return bitcoin_cash.NewAddressBuilder(cfg)
	case DriverSui:
		return sui.NewAddressBuilder(cfg)
	case DriverSubstrate:
		return substrate.NewAddressBuilder(cfg)
	case DriverTron:
		return tron.NewAddressBuilder(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
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
	}
	return xclient.UnknownError
}
