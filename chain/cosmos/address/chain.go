package address

import (
	xc "github.com/cordialsys/crosschain"
	injethsecp256k1 "github.com/cordialsys/crosschain/chain/cosmos/types/InjectiveLabs/injective-core/injective-chain/crypto/ethsecp256k1"
	"github.com/cordialsys/crosschain/chain/cosmos/types/evmos/evmos/v20/crypto/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

func IsEVMOS(asset *xc.ChainConfig) bool {
	return xc.Driver(asset.Driver) == xc.DriverCosmosEvmos
}

func GetPublicKey(asset *xc.ChainConfig, publicKeyBytes []byte) cryptotypes.PubKey {
	if asset.Chain == xc.INJ {
		// injective has their own ethsecp256k1 type..
		return &injethsecp256k1.PubKey{Key: publicKeyBytes}
	}
	if IsEVMOS(asset) {
		return &ethsecp256k1.PubKey{Key: publicKeyBytes}
	}
	return &secp256k1.PubKey{Key: publicKeyBytes}
}
