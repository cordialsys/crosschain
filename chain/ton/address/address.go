package address

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

// TON prescibes using this subwallet for importing compatibility
const DefaultSubwalletId = 698983191

// Most stable TON wallet version
const DefaultWalletVersion = wallet.V3

// AddressBuilder for Template
type AddressBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xc.AddressBuilder = AddressBuilder{}

// NewAddressBuilder creates a new Template AddressBuilder
func NewAddressBuilder(cfgI *xc.ChainBaseConfig) (xc.AddressBuilder, error) {
	return AddressBuilder{cfgI}, nil
}

// GetAddressFromPublicKey returns an Address given a public key
func (ab AddressBuilder) GetAddressFromPublicKey(publicKeyBytes []byte) (xc.Address, error) {
	addr, err := wallet.AddressFromPubKey(publicKeyBytes, DefaultWalletVersion, DefaultSubwalletId)
	if err != nil {
		return "", err
	}
	if ab.Asset.Network == "testnet" {
		addr.SetTestnetOnly(true)
	}
	return xc.Address(addr.String()), nil
}

func ParseAddress(addr xc.Address, net string) (*address.Address, error) {
	addrS := string(addr)
	if len(strings.Split(addrS, ":")) == 2 {
		addr, err := address.ParseRawAddr(addrS)
		if err == nil {
			if net == "testnet" {
				addr.SetTestnetOnly(true)
			}
		}
		return addr, err
	}

	return address.ParseAddr(addrS)
}

func Normalize(addressS string) (string, error) {
	addr, err := address.ParseAddr(addressS)
	if err != nil {
		return addressS, err
	}
	return addr.String(), nil
}

// You can calculate the token wallet address for a Jetton contract, but it appears that
// Some Jetton contracts are "legacy" and calculate their token wallet address differently.
// I couldn't find a way to distinguish between the two, so we just return both.
func CalculatePossibleTokenWalletAddresses(ownerAddr xc.Address, contractAddress xc.ContractAddress, jettonWalletCode []byte) ([]xc.Address, error) {
	v2, err := CalculateTokenWalletAddressV2(ownerAddr, contractAddress, jettonWalletCode)
	if err != nil {
		return nil, err
	}
	legacy, err := CalculateTokenWalletAddressLegacy(ownerAddr, contractAddress, jettonWalletCode)
	if err != nil {
		return nil, err
	}
	return []xc.Address{v2, legacy}, nil
}

// See https://github.com/ton-blockchain/ton/blob/2a68c8610bf28b43b2019a479a70d0606c2a0aa1/crypto/func/auto-tests/legacy_tests/jetton-wallet/imports/jetton-utils.fc#L27
func CalculateTokenWalletAddressLegacy(ownerAddr xc.Address, contractAddress xc.ContractAddress, jettonWalletCode []byte) (xc.Address, error) {
	ownerAddress, err := ParseAddress(ownerAddr, "")
	if err != nil {
		return "", fmt.Errorf("invalid owner address: %v", err)
	}
	mintAddress, err := ParseAddress(xc.Address(contractAddress), "")
	if err != nil {
		return "", fmt.Errorf("invalid contract address: %v", err)
	}
	walletCodeCell, err := cell.FromBOC(jettonWalletCode)
	if err != nil {
		return "", fmt.Errorf("invalid jetton wallet code: %v", err)
	}

	dataBuilder := cell.BeginCell()

	err = dataBuilder.StoreCoins(0)
	if err != nil {
		return "", err
	}

	err = dataBuilder.StoreAddr(ownerAddress)
	if err != nil {
		return "", err
	}

	err = dataBuilder.StoreAddr(mintAddress)
	if err != nil {
		return "", err
	}

	err = dataBuilder.StoreRef(walletCodeCell)
	if err != nil {
		return "", err
	}

	dataCell := dataBuilder.EndCell()

	stateInitBuilder := cell.BeginCell()
	err = stateInitBuilder.StoreUInt(0, 2)
	if err != nil {
		return "", err
	}

	err = stateInitBuilder.StoreMaybeRef(walletCodeCell)
	if err != nil {
		return "", err
	}

	err = stateInitBuilder.StoreMaybeRef(dataCell)
	if err != nil {
		return "", err
	}
	err = stateInitBuilder.StoreUInt(0, 1)
	if err != nil {
		return "", err
	}
	stateInitCell := stateInitBuilder.EndCell()
	hash := stateInitCell.Hash()
	walletAddress := address.NewAddress(0, 0, hash).String()
	return xc.Address(walletAddress), nil
}

// See https://docs.ton.org/v3/guidelines/dapps/cookbook#how-to-calculate-users-jetton-wallet-address-offline
func CalculateTokenWalletAddressV2(ownerAddr xc.Address, contractAddress xc.ContractAddress, jettonWalletCode []byte) (xc.Address, error) {
	ownerAddress, err := ParseAddress(ownerAddr, "")
	if err != nil {
		return "", fmt.Errorf("invalid owner address: %v", err)
	}
	mintAddress, err := ParseAddress(xc.Address(contractAddress), "")
	if err != nil {
		return "", fmt.Errorf("invalid contract address: %v", err)
	}
	walletCodeCell, err := cell.FromBOC(jettonWalletCode)
	if err != nil {
		return "", fmt.Errorf("invalid jetton wallet code: %v", err)
	}

	dataBuilder := cell.BeginCell()
	err = dataBuilder.StoreUInt(0, 4)
	if err != nil {
		return "", err
	}
	err = dataBuilder.StoreCoins(0)
	if err != nil {
		return "", err
	}

	err = dataBuilder.StoreAddr(ownerAddress)
	if err != nil {
		return "", err
	}

	err = dataBuilder.StoreAddr(mintAddress)
	if err != nil {
		return "", err
	}

	dataCell := dataBuilder.EndCell()

	stateInitBuilder := cell.BeginCell()
	err = stateInitBuilder.StoreUInt(0, 2)
	if err != nil {
		return "", err
	}

	err = stateInitBuilder.StoreMaybeRef(walletCodeCell)
	if err != nil {
		return "", err
	}

	err = stateInitBuilder.StoreMaybeRef(dataCell)
	if err != nil {
		return "", err
	}
	err = stateInitBuilder.StoreUInt(0, 1)
	if err != nil {
		return "", err
	}
	stateInitCell := stateInitBuilder.EndCell()
	hash := stateInitCell.Hash()
	walletAddress := address.NewAddress(0, 0, hash).String()
	return xc.Address(walletAddress), nil
}
