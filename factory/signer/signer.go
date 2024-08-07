package signer

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	cosmostypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	cosmoscrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/crypto"
)

// Reference implementation to sign transactions - not meant to be used for production
type Signer struct {
	driver     xc.Driver
	privateKey []byte
}

// PrivateKey is a private key or reference to private key
type PrivateKey []byte

// PublicKey is a public key
type PublicKey []byte

func fromMnemonic(privateKeyOrMnemonic string, hdPathNum uint32) (PrivateKey, error) {
	if strings.Contains(privateKeyOrMnemonic, " ") {
		if len(privateKeyOrMnemonic) < 16 {
			return nil, errors.New("invalid mnemonic")
		}
		codec := cosmostypes.MakeCosmosConfig().Marshaler
		kb := keyring.NewInMemory(codec)
		// common path, will not be correct for all chains
		hdPath := hd.CreateHDPath(hdPathNum, 0, 0).String()
		_, err := kb.NewAccount("test", privateKeyOrMnemonic, keyring.DefaultBIP39Passphrase, hdPath, hd.Secp256k1)
		if err != nil {
			return PrivateKey{}, err
		}
		armored, err := kb.ExportPrivKeyArmor("test", keyring.DefaultBIP39Passphrase)
		if err != nil {
			return PrivateKey{}, err
		}
		privkey, _, err := cosmoscrypto.UnarmorDecryptPrivKey(armored, keyring.DefaultBIP39Passphrase)
		if err != nil {
			return PrivateKey{}, err
		}
		return PrivateKey(privkey.Bytes()), nil
	} else {
		return nil, errors.New("invalid mnemonic")
	}
}

func fromString(secret string, hdNumMaybe uint32) ([]byte, error) {
	// try mnemonic first
	bz, err := fromMnemonic(secret, hdNumMaybe)
	if err != nil {
		// Try hex next
		bz, err := hex.DecodeString(secret)
		if err != nil {
			// try base58
			base58bz := base58.Decode(secret)
			return base58bz, nil
		}
		return bz, nil
	}
	return bz, nil
}

func New(driver xc.Driver, secret string, cfgMaybe *xc.ChainConfig) (*Signer, error) {
	hdNum := uint32(118)
	if cfgMaybe != nil {
		hdNum = cfgMaybe.ChainCoinHDPath
	}
	secretBz, err := fromString(secret, hdNum)
	if err != nil {
		return nil, fmt.Errorf("expected private key to be a hex or base58 string")
	}
	alg := driver.SignatureAlgorithm()
	switch alg {
	case xc.Ed255:
		if len(secretBz) == ed25519.SeedSize {
			key := ed25519.NewKeyFromSeed(secretBz)
			return &Signer{driver, key}, nil
		}
		if len(secretBz) == ed25519.PrivateKeySize {
			return &Signer{driver, secretBz}, nil
		}
		return nil, errors.New("expected ed25519 key to be 64 or 32 bytes")
	case xc.K256Keccak, xc.K256Sha256:
		_, err := crypto.HexToECDSA(hex.EncodeToString(secretBz))
		if err != nil {
			return nil, err
		}
		return &Signer{driver, secretBz}, nil
	default:
		return nil, fmt.Errorf("unsupported signing alg: %v", alg)
	}
}

func (s *Signer) Sign(data xc.TxDataToSign) (xc.TxSignature, error) {
	switch s.driver.SignatureAlgorithm() {
	case xc.Ed255:
		signatureRaw := ed25519.Sign(ed25519.PrivateKey(s.privateKey), []byte(data))
		return xc.TxSignature(signatureRaw), nil
	case xc.K256Keccak, xc.K256Sha256:
		ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(s.privateKey))
		if err != nil {
			return []byte{}, err
		}
		signatureRaw, err := crypto.Sign([]byte(data), ecdsaKey)
		return xc.TxSignature(signatureRaw), err
	default:
		return nil, fmt.Errorf("unsupported signing alg for driver: %v", s.driver)
	}
}

func (s *Signer) SignAll(data []xc.TxDataToSign) ([]xc.TxSignature, error) {
	signatures := make([]xc.TxSignature, len(data))
	for i, d := range data {
		sig, err := s.Sign(d)
		if err != nil {
			return nil, err
		}
		signatures[i] = sig
	}
	return signatures, nil
}
func (s *Signer) MustSignAll(data []xc.TxDataToSign) []xc.TxSignature {
	signatures, err := s.SignAll(data)
	if err != nil {
		panic(err)
	}
	return signatures

}
func (s *Signer) PublicKey() (PublicKey, error) {
	switch s.driver.SignatureAlgorithm() {
	case xc.Ed255:
		privateKey := ed25519.PrivateKey(s.privateKey)
		publicKey := privateKey.Public().(ed25519.PublicKey)
		return PublicKey(publicKey), nil
	case xc.K256Keccak, xc.K256Sha256:
		// _, pub := btcec.PrivKeyFromBytes(privateKey)
		ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(s.privateKey))
		if err != nil {
			return []byte{}, err
		}
		switch s.driver.PublicKeyFormat() {
		case xc.Compressed:
			return crypto.CompressPubkey(&ecdsaKey.PublicKey), nil
		default:
			return crypto.FromECDSAPub(&ecdsaKey.PublicKey), nil
		}

	default:
		return nil, fmt.Errorf("unsupported alg for driver: %v", s.driver)
	}
}
func (s *Signer) MustPublicKey() PublicKey {
	pub, err := s.PublicKey()
	if err != nil {
		panic(err)
	}
	return pub
}
