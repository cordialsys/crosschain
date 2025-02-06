package signer

import (
	"bufio"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
	cosmostypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	cosmoscrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"
)

// Reference implementation to sign transactions - not meant to be used for production
type Signer struct {
	driver      xc.Driver
	privateKey  []byte
	algorithm   xc.SignatureType
	interactive bool
}

// PrivateKey is a private key or reference to private key
type PrivateKey []byte

// PublicKey is a public key
type PublicKey []byte

const EnvEd25519ScalarSigning = "XC_SIGN_WITH_SCALAR"

const EnvPrivateKey = "XC_PRIVATE_KEY"

func ReadPrivateKeyEnv() string {
	val := os.Getenv(EnvPrivateKey)
	if val != "" {
		return val
	}
	// fallback to old PRIVATE_KEY
	return os.Getenv("PRIVATE_KEY")
}

func fromMnemonic(privateKeyOrMnemonic string, hdPathNum uint32) (PrivateKey, error) {
	if strings.Contains(privateKeyOrMnemonic, " ") {
		if len(privateKeyOrMnemonic) < 16 {
			return nil, errors.New("invalid mnemonic")
		}
		encoding, err := cosmostypes.MakeCosmosConfig(&xc.ChainConfig{ChainPrefix: "any"})
		if err != nil {
			return PrivateKey{}, err
		}
		codec := encoding.Marshaler
		kb := keyring.NewInMemory(codec)
		// common path, will not be correct for all chains
		hdPath := hd.CreateHDPath(hdPathNum, 0, 0).String()
		_, err = kb.NewAccount("test", privateKeyOrMnemonic, keyring.DefaultBIP39Passphrase, hdPath, hd.Secp256k1)
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
	secret = strings.TrimSpace(secret)
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

func New(driver xc.Driver, secret string, cfgMaybe *xc.ChainConfig, interactive bool, options ...address.AddressOption) (*Signer, error) {

	opts, err := address.NewAddressOptions(options...)
	if err != nil {
		return nil, errors.New("invalid address options")
	}
	alg := driver.SignatureAlgorithm()
	algorithmOverride, ok := opts.GetAlgorithmType()
	if ok {
		alg = algorithmOverride
	}

	if interactive {
		return &Signer{driver, []byte{}, alg, true}, nil
	}
	hdNum := uint32(118)
	if cfgMaybe != nil {
		hdNum = cfgMaybe.ChainCoinHDPath
	}
	secretBz, err := fromString(secret, hdNum)
	if err != nil {
		return nil, fmt.Errorf("expected private key to be a hex or base58 string")
	}

	switch alg {
	case xc.Ed255:
		if val := os.Getenv(EnvEd25519ScalarSigning); val == "1" || val == "true" {
			if len(secretBz) != 32 {
				return nil, fmt.Errorf("scalar must be 32 bytes, got %d bytes", len(secretBz))
			}
			return &Signer{driver, secretBz, alg, false}, nil
		}
		if len(secretBz) == ed25519.SeedSize {
			key := ed25519.NewKeyFromSeed(secretBz)
			return &Signer{driver, key, alg, false}, nil
		}
		if len(secretBz) == ed25519.PrivateKeySize {
			return &Signer{driver, secretBz, alg, false}, nil
		}
		return nil, errors.New("expected ed25519 key to be 64 or 32 bytes")
	case xc.K256Keccak, xc.K256Sha256, xc.Schnorr:
		_, err := crypto.HexToECDSA(hex.EncodeToString(secretBz))
		if err != nil {
			return nil, err
		}
		return &Signer{driver, secretBz, alg, false}, nil
	default:
		return nil, fmt.Errorf("unsupported signing alg: %v", alg)
	}
}

func (s *Signer) Sign(data xc.TxDataToSign) (xc.TxSignature, error) {
	if s.interactive {
		fmt.Println("Payload: ", hex.EncodeToString(data))
		fmt.Printf("Enter signature in hex: ")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		fmt.Println()
		bz, err := hex.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("invalid input: %v", err)
		}
		return bz, nil
	}

	switch s.algorithm {
	case xc.Ed255:
		var signatureRaw []byte
		if val := os.Getenv(EnvEd25519ScalarSigning); val == "1" || val == "true" {
			logrus.Debug("using raw scalar signing for ed25519 key")
			signatureRaw = SignWithScalar(s.privateKey, []byte(data))
		} else {
			signatureRaw = ed25519.Sign(ed25519.PrivateKey(s.privateKey), []byte(data))
		}
		return xc.TxSignature(signatureRaw), nil
	case xc.K256Keccak, xc.K256Sha256:
		ecdsaKey, err := crypto.HexToECDSA(hex.EncodeToString(s.privateKey))
		if err != nil {
			return []byte{}, err
		}
		signatureRaw, err := crypto.Sign([]byte(data), ecdsaKey)
		return xc.TxSignature(signatureRaw), err
	case xc.Schnorr:
		privKey, _ := btcec.PrivKeyFromBytes(s.privateKey)
		signature, err := schnorr.Sign(privKey, data)
		if err != nil {
			return nil, err
		}
		return signature.Serialize(), nil
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
	if s.interactive {
		fmt.Printf("Enter the public key of the signer in hex: ")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		fmt.Println()
		bz, err := hex.DecodeString(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("invalid input: %v", err)
		}
		return bz, nil
	}
	switch s.algorithm {
	case xc.Ed255:
		privateKey := ed25519.PrivateKey(s.privateKey)

		publicKey := privateKey.Public().(ed25519.PublicKey)
		return PublicKey(publicKey), nil
	case xc.K256Keccak, xc.K256Sha256, xc.Schnorr:
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
