package drivers

import (
	"fmt"
	"strings"

	"github.com/cordialsys/crosschain/chain/eos"
	"github.com/cordialsys/crosschain/chain/hyperliquid"
	"github.com/cordialsys/crosschain/chain/kaspa"
	kaspaaddress "github.com/cordialsys/crosschain/chain/kaspa/address"
	kaspabuilder "github.com/cordialsys/crosschain/chain/kaspa/builder"
	kaspaclient "github.com/cordialsys/crosschain/chain/kaspa/client"
	"github.com/cordialsys/crosschain/chain/substrate"
	xrpbuilder "github.com/cordialsys/crosschain/chain/xrp/builder"
	"github.com/cordialsys/crosschain/chain/zcash"
	zcashaddress "github.com/cordialsys/crosschain/chain/zcash/address"

	xc "github.com/cordialsys/crosschain"
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
	hedera "github.com/cordialsys/crosschain/chain/hedera"
	hederaaddress "github.com/cordialsys/crosschain/chain/hedera/address"
	hederabuilder "github.com/cordialsys/crosschain/chain/hedera/builder"
	hederaclient "github.com/cordialsys/crosschain/chain/hedera/client"
	hypeaddress "github.com/cordialsys/crosschain/chain/hyperliquid/address"
	hypebuilder "github.com/cordialsys/crosschain/chain/hyperliquid/builder"
	hypeclient "github.com/cordialsys/crosschain/chain/hyperliquid/client"
	icp "github.com/cordialsys/crosschain/chain/internet_computer"
	icpaddress "github.com/cordialsys/crosschain/chain/internet_computer/address"
	icpbuilder "github.com/cordialsys/crosschain/chain/internet_computer/builder"
	icpclient "github.com/cordialsys/crosschain/chain/internet_computer/client"
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

func NewClient(cfg xc.ITask, driver xc.Driver) (xclient.Client, error) {
	switch driver {
	case xc.DriverCardano:
		return cardanoclient.NewClient(cfg)
	case xc.DriverEVM:
		return evmclient.NewClient(cfg)
	case xc.DriverEVMLegacy:
		return evm_legacy.NewClient(cfg)
	case xc.DriverFilecoin:
		return filclient.NewClient(cfg)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmosclient.NewClient(cfg)
	case xc.DriverSolana:
		return solanaclient.NewClient(cfg)
	case xc.DriverAptos:
		return aptos.NewClient(cfg)
	case xc.DriverSui:
		return sui.NewClient(cfg)
	case xc.DriverBitcoin, xc.DriverBitcoinLegacy:
		return bitcoin.NewClient(cfg)
	case xc.DriverBitcoinCash:
		return bitcoin_cash.NewClient(cfg)
	case xc.DriverSubstrate:
		return substrateclient.NewClient(cfg)
	case xc.DriverTron:
		return tron.NewClient(cfg)
	case xc.DriverTon:
		return ton.NewClient(cfg)
	case xc.DriverXrp:
		return xrpclient.NewClient(cfg)
	case xc.DriverXlm:
		return xlmclient.NewClient(cfg)
	case xc.DriverDusk:
		return duskclient.NewClient(cfg)
	case xc.DriverKaspa:
		return kaspaclient.NewClient(cfg)
	case xc.DriverEOS:
		return eosclient.NewClient(cfg)
	case xc.DriverInternetComputerProtocol:
		return icpclient.NewClient(cfg)
	case xc.DriverHyperliquid:
		return hypeclient.NewClient(cfg)
	case xc.DriverZcash:
		return zcash.NewClient(cfg)
	case xc.DriverHedera:
		return hederaclient.NewClient(cfg)
	}
	return nil, fmt.Errorf("no client defined for chain: %s", string(cfg.GetChain().Chain))
}

func NewStakingClient(servicesConfig *services.ServicesConfig, cfg xc.ITask, provider xc.StakingProvider) (xclient.StakingClient, error) {
	driver := cfg.GetChain().Driver
	switch driver {
	case xc.DriverEVM:
		switch provider {
		case xc.Kiln:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return kiln.NewClient(rpcClient, cfg.GetChain(), &servicesConfig.Kiln)
		case xc.Figment:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return figment.NewClient(rpcClient, cfg.GetChain(), &servicesConfig.Figment)
		case xc.Twinstake:
			return nil, fmt.Errorf("not implemented")
		case xc.Native:
			rpcClient, err := evmclient.NewClient(cfg)
			if err != nil {
				return nil, err
			}
			return rpcClient, nil
		}
	case xc.DriverCardano:
		return cardanoclient.NewClient(cfg)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmosclient.NewClient(cfg)
	case xc.DriverSolana:
		return solanaclient.NewClient(cfg)
	case xc.DriverSubstrate:
		return substrateclient.NewClient(cfg)
	case xc.DriverEOS:
		return eosclient.NewClient(cfg)
	case xc.DriverSui:
		return sui.NewClient(cfg)
	case xc.DriverTron:
		return tron.NewClient(cfg)
	}
	return nil, fmt.Errorf("no staking client defined for %s on %s", provider, driver)
}

func NewTxBuilder(cfg *xc.ChainBaseConfig) (xcbuilder.FullTransferBuilder, error) {
	switch xc.Driver(cfg.Driver) {
	case xc.DriverCardano:
		return cardanobuilder.NewTxBuilder(cfg)
	case xc.DriverEVM:
		return evmbuilder.NewTxBuilder(cfg)
	case xc.DriverEVMLegacy:
		return evm_legacy.NewTxBuilder(cfg)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmosbuilder.NewTxBuilder(cfg)
	case xc.DriverSolana:
		return solanabuilder.NewTxBuilder(cfg)
	case xc.DriverAptos:
		return aptos.NewTxBuilder(cfg)
	case xc.DriverSui:
		return sui.NewTxBuilder(cfg)
	case xc.DriverBitcoin, xc.DriverBitcoinLegacy:
		return bitcoinbuilder.NewTxBuilder(cfg)
	case xc.DriverBitcoinCash:
		return bitcoin_cash.NewTxBuilder(cfg)
	case xc.DriverSubstrate:
		return substratebuilder.NewTxBuilder(cfg)
	case xc.DriverTron:
		return tron.NewTxBuilder(cfg)
	case xc.DriverTon:
		return ton.NewTxBuilder(cfg)
	case xc.DriverXrp:
		return xrpbuilder.NewTxBuilder(cfg)
	case xc.DriverXlm:
		return xlmbuilder.NewTxBuilder(cfg)
	case xc.DriverFilecoin:
		return filbuilder.NewTxBuilder(cfg)
	case xc.DriverDusk:
		return duskbuilder.NewTxBuilder(cfg)
	case xc.DriverKaspa:
		return kaspabuilder.NewTxBuilder(cfg)
	case xc.DriverEOS:
		return eosbuilder.NewTxBuilder(cfg)
	case xc.DriverInternetComputerProtocol:
		return icpbuilder.NewTxBuilder(cfg)
	case xc.DriverHyperliquid:
		return hypebuilder.NewTxBuilder(cfg)
	case xc.DriverZcash:
		return zcash.NewTxBuilder(cfg)
	case xc.DriverHedera:
		return hederabuilder.NewTxBuilder(cfg)
	}
	return nil, fmt.Errorf("no tx-builder defined for: %s", string(cfg.Chain))
}

func NewSigner(chain *xc.ChainBaseConfig, secret string, options ...xcaddress.AddressOption) (*signer.Signer, error) {
	return signer.New(chain.Driver, secret, chain, options...)
}

func NewAddressBuilder(cfg *xc.ChainBaseConfig, options ...xcaddress.AddressOption) (xc.AddressBuilder, error) {
	switch xc.Driver(cfg.Driver) {
	case xc.DriverDusk:
		return duskaddress.NewAddressBuilder(cfg)
	case xc.DriverEVM:
		return evmaddress.NewAddressBuilder(cfg)
	case xc.DriverEVMLegacy:
		return evm_legacy.NewAddressBuilder(cfg)
	case xc.DriverFilecoin:
		return filaddress.NewAddressBuilder(cfg, options...)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmosaddress.NewAddressBuilder(cfg)
	case xc.DriverSolana:
		return solanaaddress.NewAddressBuilder(cfg)
	case xc.DriverAptos:
		return aptos.NewAddressBuilder(cfg)
	case xc.DriverBitcoin, xc.DriverBitcoinLegacy:
		return bitcoinaddress.NewAddressBuilder(cfg, options...)
	case xc.DriverBitcoinCash:
		return bitcoin_cash.NewAddressBuilder(cfg)
	case xc.DriverSui:
		return sui.NewAddressBuilder(cfg)
	case xc.DriverSubstrate:
		return substrateaddress.NewAddressBuilder(cfg)
	case xc.DriverTron:
		return tron.NewAddressBuilder(cfg)
	case xc.DriverTon:
		return tonaddress.NewAddressBuilder(cfg)
	case xc.DriverXrp:
		return xrpaddress.NewAddressBuilder(cfg)
	case xc.DriverXlm:
		return xlmaddress.NewAddressBuilder(cfg)
	case xc.DriverCardano:
		return cardanoaddress.NewAddressBuilder(cfg, options...)
	case xc.DriverKaspa:
		return kaspaaddress.NewAddressBuilder(cfg)
	case xc.DriverEOS:
		return eosaddress.NewAddressBuilder(cfg)
	case xc.DriverInternetComputerProtocol:
		return icpaddress.NewAddressBuilder(cfg, options...)
	case xc.DriverHyperliquid:
		return hypeaddress.NewAddressBuilder(cfg)
	case xc.DriverZcash:
		return zcashaddress.NewAddressBuilder(cfg)
	case xc.DriverHedera:
		return hederaaddress.NewAddressBuilder(cfg)
	}
	return nil, fmt.Errorf("no address builder defined for: %s", string(cfg.Chain))
}

func CheckError(driver xc.Driver, err error) errors.Status {
	if err, ok := err.(*errors.Error); ok {
		return err.Status
	}
	switch driver {
	case xc.DriverCardano:
		return cardano.CheckError(err)
	case xc.DriverDusk:
		return dusk.CheckError(err)
	case xc.DriverEVM:
		return evm.CheckError(err)
	case xc.DriverEVMLegacy:
		return evm.CheckError(err)
	case xc.DriverFilecoin:
		return fil.CheckError(err)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmos.CheckError(err)
	case xc.DriverSolana:
		return solana.CheckError(err)
	case xc.DriverAptos:
		return aptos.CheckError(err)
	case xc.DriverBitcoin:
		return bitcoin.CheckError(err)
	case xc.DriverBitcoinLegacy:
		return bitcoin_cash.CheckError(err)
	case xc.DriverZcash:
		return bitcoin.CheckError(err)
	case xc.DriverBitcoinCash:
		return bitcoin_cash.CheckError(err)
	case xc.DriverSui:
		return sui.CheckError(err)
	case xc.DriverSubstrate:
		return substrate.CheckError(err)
	case xc.DriverTron:
		return tron.CheckError(err)
	case xc.DriverTon:
		return ton.CheckError(err)
	case xc.DriverXrp:
		return xrp.CheckError(err)
	case xc.DriverXlm:
		return xlm.CheckError(err)
	case xc.DriverKaspa:
		return kaspa.CheckError(err)
	case xc.DriverEOS:
		return eos.CheckError(err)
	case xc.DriverInternetComputerProtocol:
		return icp.CheckError(err)
	case xc.DriverHedera:
		return hedera.CheckError(err)
	}
	return errors.UnknownError
}

var ErrNoAddressValidation = fmt.Errorf("no address validation defined for")

func ValidateAddress(cfg *xc.ChainBaseConfig, addr xc.Address) error {
	if strings.TrimSpace(string(addr)) == "" {
		return fmt.Errorf("empty address")
	}
	switch cfg.Driver {
	case xc.DriverCardano:
		return cardano.ValidateAddress(cfg, addr)
	case xc.DriverDusk:
		return dusk.ValidateAddress(cfg, addr)
	case xc.DriverEVM:
		return evm.ValidateAddress(cfg, addr)
	case xc.DriverEVMLegacy:
		return evm.ValidateAddress(cfg, addr)
	case xc.DriverHyperliquid:
		return hyperliquid.ValidateAddress(cfg, addr)
	case xc.DriverFilecoin:
		return fil.ValidateAddress(cfg, addr)
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmos.ValidateAddress(cfg, addr)
	case xc.DriverSolana:
		return solana.ValidateAddress(cfg, addr)
	case xc.DriverAptos:
		return aptos.ValidateAddress(cfg, addr)
	case xc.DriverBitcoin:
		return bitcoin.ValidateAddress(cfg, addr)
	case xc.DriverBitcoinLegacy:
		return bitcoin_cash.ValidateAddress(cfg, addr)
	case xc.DriverZcash:
		return zcash.ValidateAddress(cfg, addr)
	case xc.DriverBitcoinCash:
		return bitcoin_cash.ValidateAddress(cfg, addr)
	case xc.DriverSui:
		return sui.ValidateAddress(cfg, addr)
	case xc.DriverSubstrate:
		return substrate.ValidateAddress(cfg, addr)
	case xc.DriverTron:
		return tron.ValidateAddress(cfg, addr)
	case xc.DriverTon:
		return ton.ValidateAddress(cfg, addr)
	case xc.DriverXrp:
		return xrp.ValidateAddress(cfg, addr)
	case xc.DriverXlm:
		return xlm.ValidateAddress(cfg, addr)
	case xc.DriverKaspa:
		return kaspa.ValidateAddress(cfg, addr)
	case xc.DriverEOS:
		return eos.ValidateAddress(cfg, addr)
	case xc.DriverInternetComputerProtocol:
		return icp.ValidateAddress(cfg, addr)
	case xc.DriverHedera:
		return hedera.ValidateAddress(cfg, addr)
	}
	return fmt.Errorf("%w: %s", ErrNoAddressValidation, string(cfg.Chain))
}
