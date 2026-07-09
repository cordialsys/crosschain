package tx

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

// A valid serialized ring-4 CLSAG (I || s[0..3] || c1 || D), 224 bytes, from the
// pinned known-answer vector (see crypto/clsag_vector_test.go).
const pinnedRing4CLSAG = "43a6d0948437599dec4b56a682bac61a9f84465edb708662d21a5bf72ba2a1c6" +
	"56b5553ee82cddcaa1f3609499d71ac10f9304c25d5901935cebb7fe2aa4e208" +
	"5e28e1ec8b8adc96ca711b517359e331b14121a24fde0df776a4e9d494e9df0e" +
	"a84e7c0acd982a51eb362c5ffd835265af9b59404a785d88d83f57b4edaa5003" +
	"a30f9c071eb5048a95d64797d75d44cb2a787ffc19d7dc5a99ed029f45263206" +
	"b4d8a5b41b756c81835320835a4d7845e761bd320778d8b14256063d9f83ae0e" +
	"3c9ea39e46807cf49dca334cf453995b613fc994d2eabea7fe98b99fd6a9675f"

// TestSetSignatures_StatelessPhaseDetection pins the two-phase state machine
// against the way the engine drives it: it rebuilds a fresh Tx and replays ALL
// accumulated signatures on every round. The phase must be inferred from the
// signature count, so a fresh tx handed both phases' signatures completes
// (reaches phase 2) instead of getting stuck at phase 1 and asking for CLSAG
// signatures forever.
func TestSetSignatures_StatelessPhaseDetection(t *testing.T) {
	clsagSig, err := hex.DecodeString(pinnedRing4CLSAG)
	require.NoError(t, err)

	// Phase 1 must produce the same key image the signers embed in the
	// phase-2 CLSAG (its first 32 bytes); SetSignatures cross-checks them.
	ki := clsagSig[:32]

	newTx := func() *Tx {
		return &Tx{
			Inputs:        []TxInput{{KeyImage: make([]byte, 32)}},
			RingSize:      4,
			CLSAGContexts: []CLSAGInputContext{{}},
		}
	}

	keyImageResp := &xc.SignatureResponse{Signature: ki}
	clsagResp := &xc.SignatureResponse{Signature: clsagSig}

	// Phase 1 only: a single key image → still pending phase 2.
	tx1 := newTx()
	require.NoError(t, tx1.SetSignatures(keyImageResp))
	require.Equal(t, 1, tx1.signingPhase, "one key image should leave the tx in phase 1")
	require.Equal(t, ki, tx1.Inputs[0].KeyImage)

	// Engine replay (the regression): a FRESH tx (signingPhase==0,
	// CLSAGContexts repopulated) handed BOTH signatures must reach phase 2.
	// The old code keyed the phase off signingPhase and would stay at phase 1,
	// so AdditionalSighashes handed back the CLSAG request every round forever.
	tx2 := newTx()
	require.NoError(t, tx2.SetSignatures(keyImageResp, clsagResp))
	require.Equal(t, 2, tx2.signingPhase, "fresh tx with both signatures must complete phase 2")
	require.Len(t, tx2.CLSAGs, 1)
	require.NotNil(t, tx2.CLSAGs[0])

	// Stateful CLI path: the same Tx object across sequential calls must also
	// reach phase 2, without re-assigning or reshuffling the already-set inputs.
	tx3 := newTx()
	require.NoError(t, tx3.SetSignatures(keyImageResp))
	require.NoError(t, tx3.SetSignatures(keyImageResp, clsagResp))
	require.Equal(t, 2, tx3.signingPhase)
	require.Len(t, tx3.CLSAGs, 1)
	require.Equal(t, ki, tx3.Inputs[0].KeyImage)

	// A phase-1 key image that disagrees with the one embedded in the CLSAG
	// (poisoned phase 1, misordered responses) is rejected before broadcast.
	badKI := make([]byte, 32)
	badKI[0] = 0xab
	tx4 := newTx()
	err = tx4.SetSignatures(&xc.SignatureResponse{Signature: badKI}, clsagResp)
	require.ErrorContains(t, err, "does not match the phase-1 key image")
}
