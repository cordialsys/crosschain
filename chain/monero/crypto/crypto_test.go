package crypto

import (
	"encoding/hex"
	"testing"

	"filippo.io/edwards25519"
	"github.com/cordialsys/crosschain/chain/monero/crypto/cref"
	"github.com/stretchr/testify/require"
)

func hexDec(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// Test vectors from monero-project/tests/crypto/tests.txt
func TestHashToEC(t *testing.T) {
	vectors := []struct{ input, expected string }{
		{"da66e9ba613919dec28ef367a125bb310d6d83fb9052e71034164b6dc4f392d0", "52b3f38753b4e13b74624862e253072cf12f745d43fcfafbe8c217701a6e5875"},
		{"a7fbdeeccb597c2d5fdaf2ea2e10cbfcd26b5740903e7f6d46bcbf9a90384fc6", "f055ba2d0d9828ce2e203d9896bfda494d7830e7e3a27fa27d5eaa825a79a19c"},
		{"9ae78e5620f1c4e6b29d03da006869465b3b16dae87ab0a51f4e1b74bc8aa48b", "72d8720da66f797f55fbb7fa538af0b4a4f5930c8289c991472c37dc5ec16853"},
	}
	for _, v := range vectors {
		result := HashToEC(hexDec(t, v.input))
		require.Equal(t, v.expected, hex.EncodeToString(result), "hash_to_ec(%s)", v.input)
	}
}

func TestGenerateKeyDerivation(t *testing.T) {
	vectors := []struct{ pub, sec, expected string }{
		{"fdfd97d2ea9f1c25df773ff2c973d885653a3ee643157eb0ae2b6dd98f0b6984", "eb2bd1cf0c5e074f9dbf38ebbc99c316f54e21803048c687a3bb359f7a713b02", "4e0bd2c41325a1b89a9f7413d4d05e0a5a4936f241dccc3c7d0c539ffe00ef67"},
		{"1ebf8c3c296bb91708b09d9a8e0639ccfd72556976419c7dc7e6dfd7599218b9", "e49f363fd5c8fc1f8645983647ca33d7ec9db2d255d94cd538a3cc83153c5f04", "72903ec8f9919dfcec6efb5535490527b573b3d77f9890386d373c02bf368934"},
		{"3e3047a633b1f84250ae11b5c8e8825a3df4729f6cbe4713b887db62f268187d", "6df324e24178d91c640b75ab1c6905f8e6bb275bc2c2a5d9b9ecf446765a5a05", "9dcac9c9e87dd96a4115d84d587218d8bf165a0527153b1c306e562fe39a46ab"},
	}
	for _, v := range vectors {
		result, err := GenerateKeyDerivation(hexDec(t, v.pub), hexDec(t, v.sec))
		require.NoError(t, err)
		require.Equal(t, v.expected, hex.EncodeToString(result))
	}
}

// Test vectors from monero-project/tests/crypto/tests.txt
func TestGenerateKeyImage(t *testing.T) {
	vectors := []struct{ pub, sec, expected string }{
		{"e46b60ebfe610b8ba761032018471e5719bb77ea1cd945475c4a4abe7224bfd0", "981d477fb18897fa1f784c89721a9d600bf283f06b89cb018a077f41dcefef0f", "a637203ec41eab772532d30420eac80612fce8e44f1758bc7e2cb1bdda815887"},
		{"8661153f5f856b46f83e9e225777656cd95584ab16396fa03749ec64e957283b", "156d7f2e20899371404b87d612c3587ffe9fba294bafbbc99bb1695e3275230e", "03ec63d7f1b722f551840b2725c76620fa457c805cbbf2ee941a6bf4cfb6d06c"},
		{"30216ae687676a89d84bf2a333feeceb101707193a9ee7bcbb47d54268e6cc83", "1b425ba4b8ead10f7f7c0c923ec2e6847e77aa9c7e9a880e89980178cb02fa0c", "4f675ce3a8dfd806b7c4287c19d741f51141d3fce3e3a3d1be8f3f449c22dd19"},
	}
	for _, v := range vectors {
		secScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(hexDec(t, v.sec))
		pubPoint, _ := edwards25519.NewIdentityPoint().SetBytes(hexDec(t, v.pub))
		ki := ComputeKeyImage(secScalar, pubPoint)
		require.Equal(t, v.expected, hex.EncodeToString(ki.Bytes()))
	}
}

func TestHGeneratorPoint(t *testing.T) {
	// H is a precomputed constant from Monero's crypto-ops-data.c
	require.Equal(t, "8b655970153799af2aeadc9ff1add0ea6c7251d54154cfa92c173a0dd39c1f94",
		hex.EncodeToString(H.Bytes()))
}

func TestFixedViewKey(t *testing.T) {
	// Fixed view key is deterministic from seed
	expected := ScReduce32(Keccak256([]byte("crosschain_monero_view_key")))
	require.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(FixedPrivateViewKey))

	// Calling DeriveViewKey with ANY spend key returns the fixed view key
	randomSpend := ScReduce32(Keccak256([]byte("random spend key")))
	viewKey := DeriveViewKey(randomSpend)
	require.Equal(t, hex.EncodeToString(FixedPrivateViewKey), hex.EncodeToString(viewKey))
}

func TestDeriveKeysAndAddress(t *testing.T) {
	seed := hexDec(t, "c071fe9b1096538b047087a4ee3fdae204e4682eb2dfab78f3af752704b0f700")
	privSpend, privView, pubSpend, pubView, err := DeriveKeysFromSpend(seed)
	require.NoError(t, err)
	require.Len(t, privSpend, 32)
	require.Len(t, privView, 32)
	require.Len(t, pubSpend, 32)
	require.Len(t, pubView, 32)

	// View key should be the fixed key, not derived from spend
	require.Equal(t, hex.EncodeToString(FixedPrivateViewKey), hex.EncodeToString(privView))

	// Address roundtrip
	addr := GenerateAddress(pubSpend, pubView)
	require.True(t, addr[0] == '4', "mainnet address starts with 4")
	require.Len(t, addr, 95)

	prefix, decodedSpend, decodedView, err := DecodeAddress(addr)
	require.NoError(t, err)
	require.Equal(t, MainnetAddressPrefix, prefix)
	require.Equal(t, hex.EncodeToString(pubSpend), hex.EncodeToString(decodedSpend))
	require.Equal(t, hex.EncodeToString(pubView), hex.EncodeToString(decodedView))

	// Testnet address
	testAddr := GenerateAddressWithPrefix(TestnetAddressPrefix, pubSpend, pubView)
	require.True(t, testAddr[0] == '9', "testnet address starts with 9")
}

func TestTransactionHashThreeHash(t *testing.T) {
	// Verified against real Monero testnet transaction
	// TX hash: 197d45b6a07c9ccafb7cf8e5f72c18edff1c294f1490b7dd34c8d3ff0e669814
	// The three-hash structure: H(H(prefix) || H(rct_base) || H(rct_prunable))
	prefixHash := hexDec(t, "3ba6d564a24994fe8e7ca5553f5b8cded5cddbee8dd7210f4089a89dbbeb3d0f")
	rctBaseHash := hexDec(t, "609bb65a0e2c02f10f01cc09dc658771aae5f7c019f2375532a15887b8497b3d")
	prunableHash := hexDec(t, "98228265520f9883f37590d032949760c4176c7ea0c4ae3318c06f063d9cfa75")

	combined := make([]byte, 0, 96)
	combined = append(combined, prefixHash...)
	combined = append(combined, rctBaseHash...)
	combined = append(combined, prunableHash...)
	txHash := Keccak256(combined)

	require.Equal(t, "197d45b6a07c9ccafb7cf8e5f72c18edff1c294f1490b7dd34c8d3ff0e669814",
		hex.EncodeToString(txHash))
}

func TestCommitmentMaskDerivation(t *testing.T) {
	// Verified against on-chain commitment for our mainnet deposit
	// TX: 2ed8ca963cbf3da3a8877f63d59de1d1e2055550b7c797d9f1616b5de36da10b
	// Output 0, amount: 43910000000 piconero
	// On-chain commitment: 9fae8afbe54a317a6674c06e969b93cc3944d5bc433d98a373b42294087790c7

	// Keys (from our mainnet wallet with the OLD view key derivation)
	privViewOld := hexDec(t, "639bd5b7bbbf6d0b935586331c4b9447a18d9e1450862b25ed72b5764299050b")
	txPubKey := hexDec(t, "4efc54aa09b0c7ff00e1be9628650f0ce53ffdb22c29e201a4be128ef53fa36c")

	// Derivation
	derivation, err := GenerateKeyDerivation(txPubKey, privViewOld)
	require.NoError(t, err)

	// Shared scalar
	scalar, err := DerivationToScalar(derivation, 0)
	require.NoError(t, err)

	// Commitment mask = H_s("commitment_mask" || scalar)
	data := append([]byte("commitment_mask"), scalar...)
	mask := ScReduce32(Keccak256(data))

	// Pedersen commitment: C = amount*H + mask*G
	amount := uint64(43910000000)
	commitment, err := PedersenCommit(amount, mask)
	require.NoError(t, err)

	require.Equal(t, "9fae8afbe54a317a6674c06e969b93cc3944d5bc433d98a373b42294087790c7",
		hex.EncodeToString(commitment.Bytes()))
}

func TestOutputKeyDerivationRoundtrip(t *testing.T) {
	// Test that output key derivation and scanning produce consistent results.
	// Builder derives: P = H_s(8*r*pubView || idx)*G + pubSpend
	// Scanner checks:  P == H_s(8*viewKey*R || idx)*G + pubSpend
	// Where R = r*G

	privView := FixedPrivateViewKey
	pubView, _ := PublicFromPrivate(privView)

	privSpend := ScReduce32(Keccak256([]byte("test spend key")))
	pubSpend, _ := PublicFromPrivate(privSpend)

	// Simulate builder: random tx key
	txPrivKey := ScReduce32(Keccak256([]byte("test tx key")))
	txPrivScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(txPrivKey)
	txPubKey := edwards25519.NewGeneratorPoint().ScalarBaseMult(txPrivScalar)

	// Builder output derivation (with cofactor)
	D, _ := GenerateKeyDerivation(pubView, txPrivKey)
	scalar, _ := DerivationToScalar(D, 0)
	sScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(scalar)
	sG := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar)
	pubSpendPoint, _ := edwards25519.NewIdentityPoint().SetBytes(pubSpend)
	outputKey := edwards25519.NewIdentityPoint().Add(sG, pubSpendPoint)

	// Scanner derivation (with cofactor)
	D2, _ := GenerateKeyDerivation(txPubKey.Bytes(), privView)
	scalar2, _ := DerivationToScalar(D2, 0)
	sScalar2, _ := edwards25519.NewScalar().SetCanonicalBytes(scalar2)
	sG2 := edwards25519.NewGeneratorPoint().ScalarBaseMult(sScalar2)
	expectedKey := edwards25519.NewIdentityPoint().Add(sG2, pubSpendPoint)

	require.Equal(t, 1, outputKey.Equal(expectedKey), "builder output key must match scanner derivation")

	// Amount encryption roundtrip
	amount := uint64(1000000000)
	amountBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		amountBytes[i] = byte(amount >> (8 * i))
	}
	encKey := Keccak256(append([]byte("amount"), scalar...))
	encrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		encrypted[i] = amountBytes[i] ^ encKey[i]
	}
	decKey := Keccak256(append([]byte("amount"), scalar2...))
	decrypted := make([]byte, 8)
	for i := 0; i < 8; i++ {
		decrypted[i] = encrypted[i] ^ decKey[i]
	}
	decAmount := uint64(0)
	for i := 0; i < 8; i++ {
		decAmount |= uint64(decrypted[i]) << (8 * i)
	}
	require.Equal(t, amount, decAmount, "amount must survive encrypt/decrypt roundtrip")

	// Commitment mask roundtrip
	maskData := append([]byte("commitment_mask"), scalar...)
	mask1 := ScReduce32(Keccak256(maskData))
	maskData2 := append([]byte("commitment_mask"), scalar2...)
	mask2 := ScReduce32(Keccak256(maskData2))
	require.Equal(t, hex.EncodeToString(mask1), hex.EncodeToString(mask2),
		"commitment mask must be same from builder and scanner derivation")
}

func TestBulletproofsPlusProveAndVerify(t *testing.T) {
	// Single output
	amount := uint64(1000000000) // 0.001 XMR
	mask := ScReduce32(Keccak256([]byte("test mask")))

	proof, err := cref.BPPlusProve([]uint64{amount}, [][]byte{mask})
	require.NoError(t, err)
	require.True(t, len(proof) > 500, "proof should be ~620 bytes, got %d", len(proof))

	valid := cref.BPPlusVerify(proof)
	require.True(t, valid, "BP+ proof must verify")

	// Two outputs
	mask2 := ScReduce32(Keccak256([]byte("test mask 2")))
	proof2, err := cref.BPPlusProve([]uint64{500000000, 500000000}, [][]byte{mask, mask2})
	require.NoError(t, err)
	require.True(t, cref.BPPlusVerify(proof2), "2-output BP+ proof must verify")

	// Parse proof fields
	_, fields, err := cref.ParseBPPlusProof(proof)
	require.NoError(t, err)
	require.Equal(t, 6, len(fields.L), "single output: 6 L rounds (log2(64))")
	require.Equal(t, 6, len(fields.R), "single output: 6 R rounds")

	_, fields2, err := cref.ParseBPPlusProof(proof2)
	require.NoError(t, err)
	require.Equal(t, 7, len(fields2.L), "two outputs: 7 L rounds (log2(128))")
}

func TestScalarReduce(t *testing.T) {
	// Values < L pass through
	small := hexDec(t, "0100000000000000000000000000000000000000000000000000000000000000")
	require.Equal(t, hex.EncodeToString(small), hex.EncodeToString(ScalarReduce(small)))

	// Deterministic
	hash := Keccak256([]byte("test"))
	require.Equal(t, hex.EncodeToString(ScalarReduce(hash)), hex.EncodeToString(ScalarReduce(hash)))
}
