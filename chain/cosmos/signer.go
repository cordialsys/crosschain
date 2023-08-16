package cosmos

import (
	"encoding/hex"
	"errors"
	"strings"

	cosmoscrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/crypto"
	xc "github.com/jumpcrypto/crosschain"
)

// Signer for Cosmos
type Signer struct {
	Asset xc.ITask
}

// NewSigner creates a new Cosmos Signer
func NewSigner(asset xc.ITask) (xc.Signer, error) {
	return Signer{
		Asset: asset,
	}, nil
}

// ImportPrivateKey imports a Cosmos private key
func (signer Signer) ImportPrivateKey(privateKeyOrMnemonic string) (xc.PrivateKey, error) {
	keyHex := privateKeyOrMnemonic

	if strings.Contains(privateKeyOrMnemonic, " ") {
		if len(privateKeyOrMnemonic) < 16 {
			return nil, errors.New("invalid mnemonic")
		}
		codec := MakeCosmosConfig().Marshaler
		kb := keyring.NewInMemory(codec)
		pathNum := signer.Asset.GetNativeAsset().ChainCoinHDPath
		hdPath := hd.CreateHDPath(pathNum, 0, 0).String()
		_, err := kb.NewAccount("test", privateKeyOrMnemonic, keyring.DefaultBIP39Passphrase, hdPath, hd.Secp256k1)
		if err != nil {
			return xc.PrivateKey{}, err
		}
		armored, err := kb.ExportPrivKeyArmor("test", keyring.DefaultBIP39Passphrase)
		if err != nil {
			return xc.PrivateKey{}, err
		}
		privkey, _, err := cosmoscrypto.UnarmorDecryptPrivKey(armored, keyring.DefaultBIP39Passphrase)
		if err != nil {
			return xc.PrivateKey{}, err
		}
		return xc.PrivateKey(privkey.Bytes()), nil
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	return xc.PrivateKey(key), nil
}

func (signer Signer) PublicKey(privateKey xc.PrivateKey) (xc.PublicKey, error) {
	ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(privateKey))
	if err != nil {
		return []byte{}, err
	}
	// pubkey := crypto.FromECDSAPub(&ecdsaKey.PublicKey)
	pubkey := crypto.CompressPubkey(&ecdsaKey.PublicKey)

	return pubkey, nil
}

// Sign a Cosmos tx
func (signer Signer) Sign(privateKey xc.PrivateKey, data xc.TxDataToSign) (xc.TxSignature, error) {
	privHex := hex.EncodeToString(privateKey)
	ecdsaKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return nil, err
	}
	signatureRaw, err := crypto.Sign([]byte(data), ecdsaKey)
	if err != nil {
		return nil, err
	}
	// trim off the recovery byte
	return xc.TxSignature(signatureRaw[:64]), nil
}
