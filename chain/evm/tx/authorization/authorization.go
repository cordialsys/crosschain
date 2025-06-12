package authorization

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"
)

type Authorization types.SetCodeAuthorization

func NewUnsignedAuthorization(chainId uint256.Int, address common.Address, nonce uint64) *Authorization {
	return &Authorization{
		ChainID: chainId,
		Address: address,
		Nonce:   nonce,
	}
}

func (a *Authorization) SetSignature(signature xc.TxSignature) {
	a.R = *uint256.NewInt(0).SetBytes(signature[:32])
	a.S = *uint256.NewInt(0).SetBytes(signature[32:64])
	a.V = signature[64]
}

func (a *Authorization) Sighash() []byte {
	hash := prefixedRlpHash(0x05, []any{
		a.ChainID,
		a.Address,
		a.Nonce,
	})
	return hash[:]
}

func (a *Authorization) SetCodeAuthorization() types.SetCodeAuthorization {
	return types.SetCodeAuthorization(*a)
}

func prefixedRlpHash(prefix byte, x interface{}) (h common.Hash) {
	sha := sha3.NewLegacyKeccak256()
	sha.Reset()
	sha.Write([]byte{prefix})
	rlp.Encode(sha, x)
	sha.Sum(h[:0])
	return h
}
