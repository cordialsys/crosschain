package stake_batch_deposit

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed abi.json
var abiJson string
var batchDepositAbi abi.ABI

const PublicKeyLen = 48
const CredentialLen = 32
const SignatureLen = 96

func NewAbi() abi.ABI {
	batchDeposit, err := abi.JSON(strings.NewReader(abiJson))
	if err != nil {
		panic(err)
	}
	return batchDeposit
}

func init() {
	batchDepositAbi = NewAbi()
}
func sum256(datas ...[]byte) []byte {
	h := sha256.New()
	// h := sha3.NewLegacyKeccak256()
	for _, d := range datas {
		_, _ = h.Write(d)
	}
	digest := []byte{}
	return h.Sum(digest)
}

// See here for how it's computed on EVM staking contract:
// https://etherscan.io/address/0x00000000219ab540356cBB839Cbe05303d7705Fa#code#L105
func CalculateDepositDataRoot(amount xc.AmountBlockchain, publicKey []byte, cred []byte, sig []byte) ([]byte, error) {
	if len(publicKey) != PublicKeyLen {
		return nil, fmt.Errorf("wrong length for public key, expected %d, received %d", PublicKeyLen, len(publicKey))
	}
	if len(cred) != CredentialLen {
		return nil, fmt.Errorf("wrong length for withdraw credential, expected %d, received %d", CredentialLen, len(cred))
	}
	if len(sig) != SignatureLen {
		return nil, fmt.Errorf("wrong length for signature, expected %d, received %d", SignatureLen, len(sig))
	}
	// convert to gwei
	amountBz := make([]byte, 8)
	gwei := amount.ToHuman(9).ToBlockchain(0).Uint64()
	ether := amount.ToHuman(18).ToBlockchain(0).Uint64()
	binary.LittleEndian.PutUint64(amountBz, gwei)
	if ether < 1 {
		return nil, fmt.Errorf("too low amount of ether %s", amount.ToHuman(18).String())
	}
	pubkeyRoot := sum256(publicKey, make([]byte, 16))
	signaureRoot := sum256(
		sum256(sig[:64]),
		sum256(sig[64:], make([]byte, 32)),
	)
	depositDataHash := sum256(
		sum256(pubkeyRoot, cred),
		sum256(amountBz, make([]byte, 24), signaureRoot),
	)
	return depositDataHash, nil
}

func Serialize(chainCfg *xc.ChainConfig, publicKeys [][]byte, creds [][]byte, sigs [][]byte) ([]byte, error) {
	dataHashes := make([][32]byte, len(publicKeys))
	publicKeysBz := make([]byte, len(publicKeys)*PublicKeyLen)
	credentialsBz := make([]byte, len(creds)*CredentialLen)
	sigsBz := make([]byte, len(sigs)*SignatureLen)

	if len(publicKeys) != len(creds) || len(creds) != len(sigs) || len(publicKeys) != len(sigs) {
		return nil, fmt.Errorf("not all public keys, credentials, and signatures have the same length")
	}
	amountH, _ := xc.NewAmountHumanReadableFromStr("32")
	amount := amountH.ToBlockchain(18)
	if chainCfg.Decimals > 0 {
		amount = amountH.ToBlockchain(chainCfg.Decimals)
	}

	for i := range dataHashes {
		depositDataHash, err := CalculateDepositDataRoot(amount, publicKeys[i], creds[i], sigs[i])
		if err != nil {
			return nil, err
		}
		depositHash32 := [32]byte{}
		copy(depositHash32[:], depositDataHash)
		dataHashes[i] = depositHash32

		copy(publicKeysBz[i*PublicKeyLen:], publicKeys[i])
		copy(credentialsBz[i*CredentialLen:], creds[i])
		copy(sigsBz[i*SignatureLen:], sigs[i])
	}
	bz, err := batchDepositAbi.Pack("batchDeposit", publicKeysBz, credentialsBz, sigsBz, dataHashes)
	if err != nil {
		return nil, err
	}
	return bz, nil
}
