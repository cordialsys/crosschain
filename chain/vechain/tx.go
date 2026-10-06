package vechain

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/blake2b"
)

const dynamicFeeType byte = 0x51

type clause struct {
	To    common.Address
	Value *big.Int
	Data  []byte
}

// Thor's VIP-252 field order differs from Ethereum's EIP-1559 envelope.
type envelope struct {
	ChainTag   byte
	BlockRef   uint64
	Expiration uint32
	Clauses    []clause
	GasTipCap  *big.Int
	GasFeeCap  *big.Int
	Gas        uint64
	DependsOn  []byte
	Nonce      uint64
	Reserved   []rlp.RawValue
	Signature  []byte `rlp:"optional"`
}

type Tx struct {
	envelope envelope
	sender   xc.Address
}

var _ xc.Tx = &Tx{}

func (tx *Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	env := tx.envelope
	env.Signature = nil
	payload, err := rlp.EncodeToBytes(env)
	if err != nil {
		return nil, err
	}
	hash := blake2b.Sum256(append([]byte{dynamicFeeType}, payload...))
	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash[:], tx.sender)}, nil
}

func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(signatures) != 1 || signatures[0] == nil || len(signatures[0].Signature) != crypto.SignatureLength {
		return fmt.Errorf("VeChain requires one 65-byte secp256k1 signature")
	}
	sig := signatures[0].Signature
	if !crypto.ValidateSignatureValues(sig[64], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64]), true) {
		return fmt.Errorf("invalid VeChain signature")
	}
	requests, err := tx.Sighashes()
	if err != nil {
		return err
	}
	key, err := crypto.SigToPub(requests[0].Payload, sig)
	if err != nil {
		return err
	}
	if crypto.PubkeyToAddress(*key) != common.HexToAddress(string(tx.sender)) {
		return fmt.Errorf("VeChain signature does not match sender")
	}
	tx.envelope.Signature = append([]byte(nil), sig...)
	return nil
}

func (tx *Tx) Hash() xc.TxHash {
	if len(tx.envelope.Signature) != crypto.SignatureLength {
		return ""
	}
	requests, err := tx.Sighashes()
	if err != nil {
		return ""
	}
	// The transaction ID hashes the signing hash and origin, not the signed RLP.
	hash := blake2b.Sum256(append(requests[0].Payload, common.HexToAddress(string(tx.sender)).Bytes()...))
	return xc.TxHash(hexutil.Encode(hash[:]))
}

func (tx *Tx) Serialize() ([]byte, error) {
	payload, err := rlp.EncodeToBytes(tx.envelope)
	if err != nil {
		return nil, err
	}
	return append([]byte{dynamicFeeType}, payload...), nil
}
