package signer

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcutil/base58"

	"github.com/cloudflare/circl/ecc/bls12381"
	"github.com/cloudflare/circl/sign/bls"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
	cosmostypes "github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cordialsys/crosschain/chain/dusk"
	cosmoscrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"
)

const (
	dstG1      = "BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_"
	dstG2      = "BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_"
	ScalarSize = 32
)

// Reference implementation to sign transactions - not meant to be used for production
type Signer struct {
	driver     xc.Driver
	privateKey []byte
	algorithm  xc.SignatureType
}

// PrivateKey is a private key or reference to private key
type PrivateKey []byte

// PublicKey is a public key
type PublicKey []byte

const EnvEd25519ScalarSigning = "XC_SIGN_WITH_SCALAR"

const EnvPrivateKey = "XC_PRIVATE_KEY"
const EnvPrivateKeyFeePayer = "XC_PRIVATE_KEY_FEE_PAYER"

func ReadPrivateKeyEnv() string {
	val := os.Getenv(EnvPrivateKey)
	if val != "" {
		return val
	}
	// fallback to old PRIVATE_KEY
	return os.Getenv("PRIVATE_KEY")
}

func ReadPrivateKeyFeePayerEnv() string {
	val := os.Getenv(EnvPrivateKeyFeePayer)
	if val != "" {
		return val
	}
	// fallback to old PRIVATE_KEY
	return os.Getenv("PRIVATE_KEY_FEE_PAYER")
}

func fromMnemonic(privateKeyOrMnemonic string, hdPathNum uint32) (PrivateKey, error) {
	if strings.Contains(privateKeyOrMnemonic, " ") {
		if len(privateKeyOrMnemonic) < 16 {
			return nil, errors.New("invalid mnemonic")
		}
		encoding, err := cosmostypes.MakeCosmosConfig(xc.NewChainConfig("").WithChainPrefix("any").Base())
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

func New(driver xc.Driver, secret string, cfgMaybe *xc.ChainBaseConfig, options ...address.AddressOption) (*Signer, error) {
	hdNum := uint32(118)
	if cfgMaybe != nil {
		hdNum = cfgMaybe.ChainCoinHDPath
	}
	secretBz, err := fromString(secret, hdNum)
	if err != nil {
		return nil, fmt.Errorf("expected private key to be a hex or base58 string")
	}

	opts, err := address.NewAddressOptions(options...)
	if err != nil {
		return nil, errors.New("invalid address options")
	}
	alg := driver.SignatureAlgorithm()
	algorithmOverride, ok := opts.GetAlgorithmType()
	if ok {
		alg = algorithmOverride
	}

	switch alg {
	case xc.Ed255:
		if val := os.Getenv(EnvEd25519ScalarSigning); val == "1" || val == "true" {
			if len(secretBz) != 32 {
				return nil, fmt.Errorf("scalar must be 32 bytes, got %d bytes", len(secretBz))
			}
			return &Signer{driver, secretBz, alg}, nil
		}
		if len(secretBz) == ed25519.SeedSize {
			key := ed25519.NewKeyFromSeed(secretBz)
			return &Signer{driver, key, alg}, nil
		}
		if len(secretBz) == ed25519.PrivateKeySize {
			return &Signer{driver, secretBz, alg}, nil
		}
		return nil, errors.New("expected ed25519 key to be 64 or 32 bytes")
	case xc.K256Keccak, xc.K256Sha256, xc.Schnorr:
		_, err := crypto.HexToECDSA(hex.EncodeToString(secretBz))
		if err != nil {
			return nil, err
		}
		return &Signer{driver, secretBz, alg}, nil
	case xc.Bls12_381G2Blake2:
		if len(secretBz) != 32 {
			return nil, fmt.Errorf("scalar must be 32 bytes, got %d bytes", len(secretBz))
		}
		var privKey bls.PrivateKey[bls.G2]
		err := privKey.UnmarshalBinary(secretBz)
		if err != nil && strings.Contains(err.Error(), "value out of range") {
			logrus.Warn("scalar is not on bls12-381 curve, truncating first byte")
			secretBz[0] = 0
		}
		return &Signer{driver, secretBz, alg}, nil
	default:
		return nil, fmt.Errorf("unsupported signing alg: %v", alg)
	}
}

func (s *Signer) Sign(data xc.TxDataToSign) (xc.TxSignature, error) {
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
	case xc.Bls12_381G2Blake2:
		var privKey bls.PrivateKey[bls.G2]
		err := privKey.UnmarshalBinary(s.privateKey)
		if err != nil {
			return nil, err
		}
		// bls.

		// Hash the data
		scalar, err := dusk.Blake2bScalarReduce(data)
		if err != nil {
			return nil, fmt.Errorf("failed to get scalar from sighash: %v", err)
		}
		bytes, err := scalar.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal scalar: %v", err)
		}

		Q := bls12381.G1Generator()
		Q = dusk.ScalarMultShort(bytes, Q)
		Q = dusk.ScalarMultShort(s.privateKey, Q)
		signBytes := Q.BytesCompressed()

		return xc.TxSignature(signBytes), nil
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
	case xc.Bls12_381G2Blake2:
		var blsKey bls.PrivateKey[bls.G2]
		blsKey.UnmarshalBinary(s.privateKey)
		return blsKey.PublicKey().MarshalBinary()
	default:
		return nil, fmt.Errorf("unsupported alg %v for driver: %v", s.algorithm, s.driver)
	}
}
func (s *Signer) MustPublicKey() PublicKey {
	pub, err := s.PublicKey()
	if err != nil {
		panic(err)
	}
	return pub
}
