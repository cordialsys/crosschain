package crypto

import (
	"encoding/binary"
	"fmt"

	"filippo.io/edwards25519"
)

// SubaddressIndex represents a Monero subaddress index (major account, minor address)
type SubaddressIndex struct {
	Major uint32
	Minor uint32
}

// DeriveSubaddressKeys generates a subaddress spend and view public key for the given index.
//
// Monero subaddress derivation:
//  1. m = H_s("SubAddr\0" || privateViewKey || major_index_le32 || minor_index_le32)
//  2. M = m * G
//  3. D = publicSpendKey + M   (subaddress spend public key)
//  4. C = privateViewKey * D   (subaddress view public key)
func DeriveSubaddressKeys(privateViewKey, publicSpendKey []byte, index SubaddressIndex) (subSpendPub, subViewPub []byte, err error) {
	// Special case: (0, 0) is the main address
	if index.Major == 0 && index.Minor == 0 {
		viewPub, err := PublicFromPrivate(privateViewKey)
		if err != nil {
			return nil, nil, err
		}
		return publicSpendKey, viewPub, nil
	}

	// 1. Compute m = H_s("SubAddr\0" || privateViewKey || major || minor)
	// "SubAddr\0" is 8 bytes (including the null terminator)
	data := make([]byte, 0, 8+32+4+4)
	data = append(data, []byte("SubAddr\x00")...)
	data = append(data, privateViewKey...)
	majorBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(majorBytes, index.Major)
	data = append(data, majorBytes...)
	minorBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(minorBytes, index.Minor)
	data = append(data, minorBytes...)

	mHash := Keccak256(data)
	m := ScalarReduce(mHash)

	mScalar, err := edwards25519.NewScalar().SetCanonicalBytes(m)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid subaddress scalar: %w", err)
	}

	// 2. M = m * G
	M := edwards25519.NewGeneratorPoint().ScalarBaseMult(mScalar)

	// 3. D = publicSpendKey + M
	A, err := edwards25519.NewIdentityPoint().SetBytes(publicSpendKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid public spend key: %w", err)
	}
	D := edwards25519.NewIdentityPoint().Add(A, M)
	subSpendPub = D.Bytes()

	// 4. C = privateViewKey * D
	b, err := edwards25519.NewScalar().SetCanonicalBytes(privateViewKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid private view key: %w", err)
	}
	C := edwards25519.NewIdentityPoint().ScalarMult(b, D)
	subViewPub = C.Bytes()

	return subSpendPub, subViewPub, nil
}

// GenerateSubaddress generates a Monero subaddress string for the given index.
func GenerateSubaddress(privateViewKey, publicSpendKey []byte, index SubaddressIndex) (string, error) {
	if index.Major == 0 && index.Minor == 0 {
		viewPub, err := PublicFromPrivate(privateViewKey)
		if err != nil {
			return "", err
		}
		return GenerateAddress(publicSpendKey, viewPub), nil
	}

	subSpend, subView, err := DeriveSubaddressKeys(privateViewKey, publicSpendKey, index)
	if err != nil {
		return "", err
	}
	return GenerateAddressWithPrefix(MainnetSubaddressPrefix, subSpend, subView), nil
}

// ScanOutputForSubaddresses checks if a transaction output belongs to any of the given subaddresses.
// It checks the main address and all provided subaddress spend keys.
//
// For main address: P == H_s(derivation || idx)*G + pubSpendKey
// For subaddress:   P - H_s(derivation || idx)*G should match a known subaddress spend key
//
// Returns: (matched, subaddressIndex, amount, error)
// If matched against main address, subaddressIndex will be {0, 0}.
func ScanOutputForSubaddresses(
	txPubKey []byte,
	outputIndex uint64,
	outputKeyHex string,
	encryptedAmountHex string,
	privateViewKey []byte,
	publicSpendKey []byte,
	subaddressSpendKeys map[SubaddressIndex][]byte, // maps subaddress index -> subaddress spend public key
) (matched bool, matchedIndex SubaddressIndex, amount uint64, err error) {
	outputKeyBytes, err := decodeHex(outputKeyHex)
	if err != nil {
		return false, SubaddressIndex{}, 0, fmt.Errorf("invalid output key: %w", err)
	}

	// 1. Generate key derivation: D = 8 * viewKey * txPubKey
	derivation, err := GenerateKeyDerivation(txPubKey, privateViewKey)
	if err != nil {
		return false, SubaddressIndex{}, 0, fmt.Errorf("key derivation failed: %w", err)
	}

	// 2. Compute scalar: s = H_s(D || outputIndex)
	scalar, err := DerivationToScalar(derivation, outputIndex)
	if err != nil {
		return false, SubaddressIndex{}, 0, fmt.Errorf("derivation to scalar failed: %w", err)
	}

	// 3. Compute s*G
	sScalar, err := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	if err != nil {
		return false, SubaddressIndex{}, 0, fmt.Errorf("invalid scalar: %w", err)
	}
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)

	// 4. Compute P - s*G
	P, err := edwards25519.NewIdentityPoint().SetBytes(outputKeyBytes)
	if err != nil {
		return false, SubaddressIndex{}, 0, fmt.Errorf("invalid output key point: %w", err)
	}
	negSG := edwards25519.NewIdentityPoint().Negate(sG)
	candidate := edwards25519.NewIdentityPoint().Add(P, negSG)
	candidateBytes := candidate.Bytes()

	// 5. Check against main address spend key
	if bytesEqual(candidateBytes, publicSpendKey) {
		if encryptedAmountHex != "" {
			amount, err = DecryptAmount(encryptedAmountHex, scalar)
			if err != nil {
				return true, SubaddressIndex{Major: 0, Minor: 0}, 0, nil
			}
		}
		return true, SubaddressIndex{Major: 0, Minor: 0}, amount, nil
	}

	// 6. Check against all subaddress spend keys
	for idx, subSpendKey := range subaddressSpendKeys {
		if bytesEqual(candidateBytes, subSpendKey) {
			if encryptedAmountHex != "" {
				amount, err = DecryptAmount(encryptedAmountHex, scalar)
				if err != nil {
					return true, idx, 0, nil
				}
			}
			return true, idx, amount, nil
		}
	}

	return false, SubaddressIndex{}, 0, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func decodeHex(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		h := hexVal(s[i])
		l := hexVal(s[i+1])
		if h < 0 || l < 0 {
			return nil, fmt.Errorf("invalid hex character at position %d", i)
		}
		b[i/2] = byte(h<<4 | l)
	}
	return b, nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}
