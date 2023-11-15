package drivers

import (
	"errors"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/cosmos"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/chain/solana"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
)

func NewClient(cfg ITask, driver Driver) (Client, error) {
	switch driver {
	case DriverEVM:
		return evm.NewClient(cfg)
	case DriverEVMLegacy:
		return evm.NewLegacyClient(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewClient(cfg)
	case DriverSolana:
		return solana.NewClient(cfg)
	case DriverAptos:
		return aptos.NewClient(cfg)
	case DriverSui:
		return sui.NewClient(cfg)
	case DriverBitcoin:
		return bitcoin.NewClient(cfg)
	case DriverSubstrate:
		return substrate.NewClient(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewTxBuilder(cfg ITask) (TxBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM:
		return evm.NewTxBuilder(cfg)
	case DriverEVMLegacy:
		return evm.NewLegacyTxBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewTxBuilder(cfg)
	case DriverSolana:
		return solana.NewTxBuilder(cfg)
	case DriverAptos:
		return aptos.NewTxBuilder(cfg)
	case DriverSui:
		return sui.NewTxBuilder(cfg)
	case DriverBitcoin:
		return bitcoin.NewTxBuilder(cfg)
	case DriverSubstrate:
		return substrate.NewTxBuilder(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewSigner(cfg ITask) (Signer, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM, DriverEVMLegacy:
		return evm.NewSigner(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewSigner(cfg)
	case DriverSolana:
		return solana.NewSigner(cfg)
	case DriverAptos:
		return aptos.NewSigner(cfg)
	case DriverBitcoin:
		return bitcoin.NewSigner(cfg)
	case DriverSui:
		return sui.NewSigner(cfg)
	case DriverSubstrate:
		return substrate.NewSigner(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func NewAddressBuilder(cfg ITask) (AddressBuilder, error) {
	switch Driver(cfg.GetChain().Driver) {
	case DriverEVM, DriverEVMLegacy:
		return evm.NewAddressBuilder(cfg)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.NewAddressBuilder(cfg)
	case DriverSolana:
		return solana.NewAddressBuilder(cfg)
	case DriverAptos:
		return aptos.NewAddressBuilder(cfg)
	case DriverBitcoin:
		return bitcoin.NewAddressBuilder(cfg)
	case DriverSui:
		return sui.NewAddressBuilder(cfg)
	case DriverSubstrate:
		return substrate.NewAddressBuilder(cfg)
	}
	return nil, errors.New("unsupported asset: " + string(cfg.ID()))
}

func CheckError(driver Driver, err error) ClientError {
	switch driver {
	case DriverEVM, DriverEVMLegacy:
		return evm.CheckError(err)
	case DriverCosmos, DriverCosmosEvmos:
		return cosmos.CheckError(err)
	case DriverSolana:
		return solana.CheckError(err)
	case DriverAptos:
		return aptos.CheckError(err)
	case DriverBitcoin:
		return bitcoin.CheckError(err)
	case DriverSui:
		return sui.CheckError(err)
	case DriverSubstrate:
		return substrate.CheckError(err)
	}
	return UnknownError
}
