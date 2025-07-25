package certification

import (
	"bytes"
	"crypto/ed25519"
	"encoding/asn1"
	"fmt"
	"slices"
	"time"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
	"github.com/cordialsys/crosschain/chain/internet_computer/certification/hashtree"

	"github.com/fxamacker/cbor/v2"
)

func PublicBLSKeyToDER(publicKey []byte) ([]byte, error) {
	if len(publicKey) != 96 {
		return nil, fmt.Errorf("invalid public key length: %d", len(publicKey))
	}
	return asn1.Marshal([]any{
		[]any{
			asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 44668, 5, 3, 1, 2, 1}, // algorithm identifier
			asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 44668, 5, 3, 2, 1},    // curve identifier
		},
		asn1.BitString{
			Bytes:     publicKey,
			BitLength: len(publicKey) * 8,
		},
	})
}

func PublicED25519KeyFromDER(der []byte) (*ed25519.PublicKey, error) {
	var seq asn1.RawValue
	if _, err := asn1.Unmarshal(der, &seq); err != nil {
		return nil, err
	}
	if seq.Tag != asn1.TagSequence {
		return nil, fmt.Errorf("invalid tag: %d", seq.Tag)
	}
	var idSeq asn1.RawValue
	rest, err := asn1.Unmarshal(seq.Bytes, &idSeq)
	if err != nil {
		return nil, err
	}
	var bs asn1.BitString
	if _, err := asn1.Unmarshal(rest, &bs); err != nil {
		return nil, err
	}
	var algoId asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(idSeq.Bytes, &algoId); err != nil {
		return nil, err
	}
	if !algoId.Equal(asn1.ObjectIdentifier{1, 3, 101, 112}) {
		return nil, fmt.Errorf("invalid algorithm identifier: %v", algoId)
	}
	publicKey := ed25519.PublicKey(bs.Bytes)
	return &publicKey, nil
}

type CanisterRange struct {
	From address.Principal
	To   address.Principal
}

func (c *CanisterRange) UnmarshalCBOR(bytes []byte) error {
	var raw [][]byte
	if err := cbor.Unmarshal(bytes, &raw); err != nil {
		return err
	}
	if len(raw) != 2 {
		return fmt.Errorf("unexpected length: %d", len(raw))
	}
	c.From = address.Principal{Raw: raw[0]}
	c.To = address.Principal{Raw: raw[1]}
	return nil
}

type CanisterRanges []CanisterRange

func (c CanisterRanges) InRange(canisterID address.Principal) bool {
	for _, r := range c {
		if slices.Compare(r.From.Raw, canisterID.Raw) <= 0 && slices.Compare(canisterID.Raw, r.To.Raw) <= 0 {
			return true
		}
	}
	return false
}

// Certificate is a certificate gets returned by the IC.
type Certificate struct {
	// Tree is the certificate tree.
	Tree hashtree.HashTree `cbor:"tree"`
	// Signature is the signature of the certificate tree.
	Signature []byte `cbor:"signature"`
	// Delegation is the delegation of the certificate.
	Delegation *Delegation `cbor:"delegation"`
}

// VerifyTime verifies the time of a certificate.
func (c Certificate) VerifyTime(ingressExpiry time.Duration) error {
	rawTime, err := c.Tree.Lookup(hashtree.Label("time"))
	if err != nil {
		return err
	}
	t, err := leb128.DecodeUnsigned(bytes.NewReader(rawTime))
	if err != nil {
		return err
	}
	if int64(ingressExpiry) < time.Now().UnixNano()-t.Int64() {
		return fmt.Errorf("certificate outdated, exceeds ingress expiry")
	}
	return nil
}

// Delegation is a delegation of a certificate.
type Delegation struct {
	// SubnetId is the subnet ID of the delegation.
	SubnetId address.Principal `cbor:"subnet_id"`
	// The nested certificate typically does not itself again contain a
	// delegation, although there is no reason why agents should enforce that
	// property.
	Certificate Certificate `cbor:"certificate"`
}

// UnmarshalCBOR unmarshals a delegation.
func (d *Delegation) UnmarshalCBOR(bytes []byte) error {
	var m map[string][]byte
	if err := cbor.Unmarshal(bytes, &m); err != nil {
		return err
	}
	for k, v := range m {
		switch k {
		case "subnet_id":
			d.SubnetId = address.Principal{
				Raw: v,
			}
		case "certificate":
			if err := cbor.Unmarshal(v, &d.Certificate); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown key: %s", k)
		}
	}
	return nil
}
