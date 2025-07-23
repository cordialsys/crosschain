package address

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

var (
	DefaultPrincipalSubaccount [32]byte
)

// Generic ICP identifier
type Principal struct {
	Raw []byte
}

func PrincipalFromPublicKey(pk []byte) ([]byte, error) {
	der, err := DerEncodePublicKey(pk)
	if err != nil {
		return nil, fmt.Errorf("failed to der encode public key: %w", err)
	}

	hash := sha256.Sum224(der)
	return append(hash[:], 0x02), nil
}

func (p Principal) Encode() string {
	cs := make([]byte, 4)
	binary.BigEndian.PutUint32(cs, crc32.ChecksumIEEE(p.Raw))
	encoded := Encoding.EncodeToString(append(cs, p.Raw...))
	b32 := strings.ToLower(encoded)
	var str string
	for i, c := range b32 {
		if i != 0 && i%5 == 0 {
			str += "-"
		}
		str += string(c)
	}
	return str
}

func Decode(s string) (Principal, error) {
	p := strings.Split(s, "-")
	for i, c := range p {
		if len(c) > 5 {
			return Principal{}, fmt.Errorf("invalid length: %s", c)
		}
		if i != len(p)-1 && len(c) < 5 {
			return Principal{}, fmt.Errorf("invalid length: %s", c)
		}
	}
	b32, err := Encoding.DecodeString(strings.ToUpper(strings.Join(p, "")))
	if err != nil {
		return Principal{}, err
	}
	if len(b32) < 4 {
		return Principal{}, fmt.Errorf("invalid length: %s", b32)
	}
	if crc32.ChecksumIEEE(b32[4:]) != binary.BigEndian.Uint32(b32[:4]) {
		return Principal{}, fmt.Errorf("invalid checksum: %s", b32)
	}
	return Principal{b32[4:]}, err
}

// String implements the Stringer interface.
func (p Principal) String() string {
	return p.Encode()
}

func MustDecode(s string) Principal {
	p, err := Decode(s)
	if err != nil {
		panic(err)
	}

	return p
}

func (p Principal) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(p.Raw)
}

func (p Principal) UnmarshalCBOR(bytes []byte) error {
	return cbor.Unmarshal(bytes, &p.Raw)
}

// ICP supports Ed25519 and Secp256k identities
type Ed25519Identity struct {
	PublicKey ed25519.PublicKey
}

func NewEd25519Identity(pubKeyBytes []byte) Ed25519Identity {
	return Ed25519Identity{
		PublicKey: ed25519.PublicKey(pubKeyBytes),
	}
}

func (id Ed25519Identity) DerPublicKey() ([]byte, error) {
	if id.PublicKey == nil {
		return nil, nil
	}

	return DerEncodePublicKey(id.PublicKey)
}

func (id Ed25519Identity) Principal() (Principal, error) {
	derPk, err := id.DerPublicKey()
	if err != nil {
		return Principal{}, fmt.Errorf("failed to get Principal: %v", err)
	}

	if derPk == nil {
		return AnonymousPrincipal, nil
	}

	hash := sha256.Sum224(derPk)
	return Principal{
		Raw: append(hash[:], 0x02),
	}, nil
}
