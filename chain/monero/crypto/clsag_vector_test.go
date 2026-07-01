package crypto

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

// Known-answer test vectors for CLSAG signing.
//
// These vectors pin both phases of the Monero signing flow exposed by
// SignCLSAGFromPayload. Inputs are fixed; the deterministic RNG seed
// (RngSeed) makes the nonce alpha and decoy responses s[i ≠ ℓ]
// reproducible, so the entire output is byte-deterministic.
//
// Purpose: cross-implementation validation. In particular, a threshold-MPC
// CLSAG signer (FROST-adapted, see clsag-mpc-protocol.md §10.2) must
// produce byte-identical Phase 1 and Phase 2 outputs to these vectors
// when given the same inputs and the same effective nonce.
//
// To regenerate after an intentional algorithmic change, set the env
// var XC_CLSAG_PRINT_VECTORS=1 and run the test once; the failure
// output prints the new pinned bytes to copy in.

const clsagVectorMessage = "clsag-mpc-test-vector-v1"

// fixed inputs shared by phase 1 and phase 2 vector tests
type clsagVectorInputs struct {
	privSpend   []byte
	privView    []byte
	txPubKey    []byte
	txPubKeyHex string
	outputIndex uint64

	// derived
	oneTimePriv *edwards25519.Scalar
	oneTimePub  *edwards25519.Point
	outputKey   string
}

func buildCLSAGVectorInputs(t *testing.T) clsagVectorInputs {
	t.Helper()

	privSpend := hexDec(t, "0101010101010101010101010101010101010101010101010101010101010101")
	privView := hexDec(t, "0202020202020202020202020202020202020202020202020202020202020202")

	// Derive a deterministic tx pub key R = r·G from a fixed scalar
	rSeed := hexDec(t, "0303030303030303030303030303030303030303030303030303030303030303")
	rScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(rSeed))
	R := edwards25519.NewGeneratorPoint().ScalarBaseMult(rScalar)

	outputIndex := uint64(0)

	// Derive the one-time keys (same path as SignCLSAGFromPayload)
	privSpendScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(privSpend))
	derivation, err := GenerateKeyDerivation(R.Bytes(), privView)
	require.NoError(t, err)
	hsBytes, err := DerivationToScalar(derivation, outputIndex)
	require.NoError(t, err)
	hsScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(hsBytes)
	oneTimePriv := edwards25519.NewScalar().Add(hsScalar, privSpendScalar)
	oneTimePub := edwards25519.NewGeneratorPoint().ScalarBaseMult(oneTimePriv)

	return clsagVectorInputs{
		privSpend:   privSpend,
		privView:    privView,
		txPubKey:    R.Bytes(),
		txPubKeyHex: hex.EncodeToString(R.Bytes()),
		outputIndex: outputIndex,
		oneTimePriv: oneTimePriv,
		oneTimePub:  oneTimePub,
		outputKey:   hex.EncodeToString(oneTimePub.Bytes()),
	}
}

// TestCLSAGVector_DerivedOneTimeKey pins the deterministically-derived
// output key for the wallet (a, v) and tx pub key R used in the other
// vector tests. An MPC implementation must independently arrive at the
// same one-time public key from its own per-party share computation.
func TestCLSAGVector_DerivedOneTimeKey(t *testing.T) {
	in := buildCLSAGVectorInputs(t)

	const expectedTxPubKey = "a47d1c5386f1e0ad6d1f4e059a58dae483430be3eafce41e879a3b791cac2ea7"
	const expectedOutputKey = "c5e90980d9d267dd0a96011a47b2d4d83b9da42f853f5f99a269a3c315928585"

	maybePrint(t, "tx_pub_key", in.txPubKeyHex)
	maybePrint(t, "output_key", in.outputKey)

	require.Equal(t, expectedTxPubKey, in.txPubKeyHex)
	require.Equal(t, expectedOutputKey, in.outputKey)
}

// TestCLSAGVector_Phase1KeyImage pins the Phase-1 output (the 32-byte key
// image) for the fixed wallet keys and output. This is the value an MPC
// Phase-1 protocol must reconstruct by summing per-party I_j contributions.
func TestCLSAGVector_Phase1KeyImage(t *testing.T) {
	in := buildCLSAGVectorInputs(t)

	// Phase 1 payload: no Message/RingKeys, only output_key + tx_pub_key + output_index
	payload, _ := json.Marshal(MoneroSighash{
		OutputKey:   in.outputKey,
		TxPubKey:    in.txPubKeyHex,
		OutputIndex: in.outputIndex,
	})

	got, err := SignCLSAGFromPayload(payload, in.privSpend, in.privView)
	require.NoError(t, err)
	require.Len(t, got, 32, "Phase 1 returns 32-byte key image")

	const expectedKeyImage = "43a6d0948437599dec4b56a682bac61a9f84465edb708662d21a5bf72ba2a1c6"
	maybePrint(t, "key_image", hex.EncodeToString(got))

	require.Equal(t, expectedKeyImage, hex.EncodeToString(got))
}

// TestCLSAGVector_Phase2Signature pins the Phase-2 output (the full
// serialized CLSAG signature: key_image || s[0..n-1] || c1 || D).
// This is the byte-for-byte target for an MPC signer running with an
// equivalent effective nonce (i.e., t=n=1 single-party simulation or a
// real multi-party run with α = Σ r_j matching the deterministic RNG's
// first output).
func TestCLSAGVector_Phase2Signature(t *testing.T) {
	in := buildCLSAGVectorInputs(t)

	const (
		realPos = 2
		n       = 4
	)

	decoySpends := []string{
		"0404040404040404040404040404040404040404040404040404040404040404",
		"0505050505050505050505050505050505050505050505050505050505050505",
		"0606060606060606060606060606060606060606060606060606060606060606",
	}
	decoyMasks := []string{
		"0707070707070707070707070707070707070707070707070707070707070707",
		"0808080808080808080808080808080808080808080808080808080808080808",
		"0909090909090909090909090909090909090909090909090909090909090909",
	}

	ringKeys := make([]string, n)
	ringCommits := make([]string, n)
	di := 0
	for i := 0; i < n; i++ {
		if i == realPos {
			continue
		}
		s, _ := edwards25519.NewScalar().SetCanonicalBytes(ScalarReduce(hexDec(t, decoySpends[di])))
		P := edwards25519.NewGeneratorPoint().ScalarBaseMult(s)
		ringKeys[i] = hex.EncodeToString(P.Bytes())

		// Masks must be reduced canonical scalars before PedersenCommit.
		maskReduced := ScalarReduce(hexDec(t, decoyMasks[di]))
		c, err := PedersenCommit(uint64(1_000_000_000+i), maskReduced)
		require.NoError(t, err)
		ringCommits[i] = hex.EncodeToString(c.Bytes())
		di++
	}

	// Real ring slot
	ringKeys[realPos] = in.outputKey
	realAmount := uint64(5_000_000_000)
	inputMaskHex := "1010101010101010101010101010101010101010101010101010101010101010"
	pseudoMaskHex := "1111111111111111111111111111111111111111111111111111111111111111"

	inputMaskReduced := ScalarReduce(hexDec(t, inputMaskHex))
	pseudoMaskReduced := ScalarReduce(hexDec(t, pseudoMaskHex))

	realCommit, err := PedersenCommit(realAmount, inputMaskReduced)
	require.NoError(t, err)
	ringCommits[realPos] = hex.EncodeToString(realCommit.Bytes())

	cOffset, err := PedersenCommit(realAmount, pseudoMaskReduced)
	require.NoError(t, err)

	inMaskScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(inputMaskReduced)
	psMaskScalar, _ := edwards25519.NewScalar().SetCanonicalBytes(pseudoMaskReduced)
	z := edwards25519.NewScalar().Subtract(inMaskScalar, psMaskScalar)

	message := Keccak256([]byte(clsagVectorMessage))
	rngSeed := hexDec(t, "0000000000000000000000000000000000000000000000000000000000000042")

	payload, _ := json.Marshal(MoneroSighash{
		Message:         message,
		RingKeys:        ringKeys,
		RingCommitments: ringCommits,
		COffset:         hex.EncodeToString(cOffset.Bytes()),
		RealPos:         realPos,
		ZKey:            hex.EncodeToString(z.Bytes()),
		OutputKey:       in.outputKey,
		TxPubKey:        in.txPubKeyHex,
		OutputIndex:     in.outputIndex,
		RngSeed:         rngSeed,
	})

	sigBytes, err := SignCLSAGFromPayload(payload, in.privSpend, in.privView)
	require.NoError(t, err)

	// Layout: key_image(32) || s[0..n-1](32 each) || c1(32) || D(32)
	require.Len(t, sigBytes, 32+n*32+32+32)

	keyImage := sigBytes[:32]
	c1 := sigBytes[32+n*32 : 32+n*32+32]
	D := sigBytes[32+n*32+32:]
	sValues := make([]string, n)
	for i := 0; i < n; i++ {
		sValues[i] = hex.EncodeToString(sigBytes[32+i*32 : 32+(i+1)*32])
	}

	maybePrint(t, "ring[0]", ringKeys[0])
	maybePrint(t, "ring[1]", ringKeys[1])
	maybePrint(t, "ring[2]", ringKeys[2])
	maybePrint(t, "ring[3]", ringKeys[3])
	maybePrint(t, "commit[0]", ringCommits[0])
	maybePrint(t, "commit[1]", ringCommits[1])
	maybePrint(t, "commit[2]", ringCommits[2])
	maybePrint(t, "commit[3]", ringCommits[3])
	maybePrint(t, "c_offset", hex.EncodeToString(cOffset.Bytes()))
	maybePrint(t, "z", hex.EncodeToString(z.Bytes()))
	maybePrint(t, "message", hex.EncodeToString(message))
	maybePrint(t, "key_image", hex.EncodeToString(keyImage))
	maybePrint(t, "c1", hex.EncodeToString(c1))
	maybePrint(t, "D", hex.EncodeToString(D))
	for i, s := range sValues {
		maybePrint(t, "s["+string(rune('0'+i))+"]", s)
	}
	maybePrint(t, "sig_full", hex.EncodeToString(sigBytes))

	// --- Pinned expected values ---
	//
	// Layout: key_image(32) || s[0](32) || s[1](32) || s[2](32) || s[3](32) || c1(32) || D(32)
	//
	// To regenerate after intentional algorithm changes, set
	// XC_CLSAG_PRINT_VECTORS=1 and re-run; the test logs all components.
	const (
		expectedKeyImage = "43a6d0948437599dec4b56a682bac61a9f84465edb708662d21a5bf72ba2a1c6"
		expectedS0       = "56b5553ee82cddcaa1f3609499d71ac10f9304c25d5901935cebb7fe2aa4e208"
		expectedS1       = "5e28e1ec8b8adc96ca711b517359e331b14121a24fde0df776a4e9d494e9df0e"
		expectedS2       = "a84e7c0acd982a51eb362c5ffd835265af9b59404a785d88d83f57b4edaa5003"
		expectedS3       = "a30f9c071eb5048a95d64797d75d44cb2a787ffc19d7dc5a99ed029f45263206"
		expectedC1       = "b4d8a5b41b756c81835320835a4d7845e761bd320778d8b14256063d9f83ae0e"
		expectedD        = "3c9ea39e46807cf49dca334cf453995b613fc994d2eabea7fe98b99fd6a9675f"
	)
	expectedSig := expectedKeyImage + expectedS0 + expectedS1 + expectedS2 + expectedS3 + expectedC1 + expectedD

	require.Equal(t, expectedSig, hex.EncodeToString(sigBytes),
		"CLSAG signature must equal pinned vector")

	// And confirm the pinned signature verifies (defense in depth)
	ring := make([]*edwards25519.Point, n)
	commits := make([]*edwards25519.Point, n)
	for i := 0; i < n; i++ {
		rb, _ := hex.DecodeString(ringKeys[i])
		ring[i], _ = edwards25519.NewIdentityPoint().SetBytes(rb)
		cb, _ := hex.DecodeString(ringCommits[i])
		commits[i], _ = edwards25519.NewIdentityPoint().SetBytes(cb)
	}
	sig, _, err := DeserializeCLSAG(sigBytes, n)
	require.NoError(t, err)
	require.True(t, CLSAGVerify(message, ring, commits, cOffset, sig),
		"pinned CLSAG vector must verify")
}

// maybePrint emits a labelled hex value when XC_CLSAG_PRINT_VECTORS=1 is
// set. Used to regenerate pinned vectors after intentional algorithm
// changes. In normal runs it's silent so passing tests stay quiet.
func maybePrint(t *testing.T, label, hex string) {
	if os.Getenv("XC_CLSAG_PRINT_VECTORS") == "1" {
		t.Logf("VECTOR %s = %s", label, hex)
	}
}
