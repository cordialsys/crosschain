package call_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xccall "github.com/cordialsys/crosschain/call"
	"github.com/cordialsys/crosschain/chain/solana/call"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/stretchr/testify/require"
)

// helper to create a simple tx with two signers: payer (k1) and an extra signer (k2) via memo instruction
func newTwoSignerTx(t *testing.T, k1 solana.PublicKey, k2 solana.PublicKey) *solana.Transaction {
	t.Helper()
	instrs := []solana.Instruction{
		memo.NewMemoInstruction([]byte("m1"), k1).Build(),
		memo.NewMemoInstruction([]byte("m2"), k2).Build(),
	}
	recent := solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK")
	tx, err := solana.NewTransaction(instrs, recent, solana.TransactionPayer(k1))
	require.NoError(t, err)
	return tx
}

// newCustomProgramTx builds a single-instruction tx that references a non-native program ID.
// Include payer as a signer account to satisfy signer requirement.
func newCustomProgramTx(t *testing.T, payer solana.PublicKey, programID solana.PublicKey) *solana.Transaction {
	accs := []*solana.AccountMeta{
		{PublicKey: payer, IsSigner: true, IsWritable: true},
	}
	ins := solana.NewInstruction(programID, accs, []byte{0x01, 0x02})
	recent := solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK")
	tx, err := solana.NewTransaction([]solana.Instruction{ins}, recent, solana.TransactionPayer(payer))
	require.NoError(t, err)
	return tx
}

func mustMarshalCall(t *testing.T, call call.Call) json.RawMessage {
	b, err := json.Marshal(call)
	require.NoError(t, err)
	return b
}

func findSignerIndex(signers []solana.PublicKey, target solana.PublicKey) int {
	for i, s := range signers {
		if s.Equals(target) {
			return i
		}
	}
	return -1
}

func TestSetSignatures_PreservesExisting(t *testing.T) {
	k1 := solana.NewWallet().PublicKey()
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, k1, k2)
	msgBytes, err := tx.MarshalBinary()
	require.NoError(t, err, "failed to marshal tx")

	// Build Call JSON targeting k1 as the requested account
	raw := mustMarshalCall(t, call.Call{Transaction: msgBytes})
	cfg := &xc.ChainBaseConfig{}
	c, err := call.NewCall(cfg, xccall.SolanaSignTransaction, raw, xc.Address(k1.String()))
	require.NoError(t, err, "NewCall failed")

	// Map indices for k1 and k2
	signers := c.SolTx.Message.Signers()
	i1 := findSignerIndex(signers, k1)
	i2 := findSignerIndex(signers, k2)
	require.GreaterOrEqual(t, i1, 0, "expected k1 to be a signer, got: %v", signers)
	require.GreaterOrEqual(t, i2, 0, "expected k2 to be a signer, got: %v", signers)

	// First, set signature for k1 only
	sig1 := make([]byte, 64)
	for i := range sig1 {
		sig1[i] = 0x11
	}
	err = c.SetSignatures(&xc.SignatureResponse{Signature: sig1, PublicKey: k1.Bytes()})
	require.NoError(t, err, "SetSignatures failed")
	require.Len(t, c.SolTx.Signatures, len(signers), "unexpected signatures length")
	require.NotEqual(t, (solana.Signature{}).String(), c.SolTx.Signatures[i1].String(), "expected signature for k1 to be set")
	require.Equal(t, (solana.Signature{}).String(), c.SolTx.Signatures[i2].String(), "expected signature for k2 to be empty initially")

	// Now, set signature for k2 only; k1 should be preserved
	sig2 := make([]byte, 64)
	for i := range sig2 {
		sig2[i] = 0x22
	}
	err = c.SetSignatures(&xc.SignatureResponse{Signature: sig2, PublicKey: k2.Bytes()})
	require.NoError(t, err, "SetSignatures failed")
	require.NotEqual(t, (solana.Signature{}).String(), c.SolTx.Signatures[i1].String(), "expected signature for k1 to be preserved, but it was cleared")
	require.NotEqual(t, (solana.Signature{}).String(), c.SolTx.Signatures[i2].String(), "expected signature for k2 to be set, but it is empty")

	// Calling with empty slice should not wipe existing signatures
	err = c.SetSignatures()
	require.NoError(t, err, "SetSignatures(empty) failed")
	require.NotEqual(t, (solana.Signature{}).String(), c.SolTx.Signatures[i1].String(), "expected existing signatures to be preserved when passing no signatures")
	require.NotEqual(t, (solana.Signature{}).String(), c.SolTx.Signatures[i2].String(), "expected existing signatures to be preserved when passing no signatures")
}

func TestSighashesAndHashBehavior(t *testing.T) {
	k1 := solana.NewWallet().PublicKey()
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, k1, k2)
	msgBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	raw := mustMarshalCall(t, call.Call{Transaction: msgBytes})
	c, err := call.NewCall(&xc.ChainBaseConfig{}, xccall.SolanaSignTransaction, raw, xc.Address(k1.String()))
	require.NoError(t, err)

	// Hash should be empty initially
	require.Empty(t, c.Hash())

	// Sighashes should target k1 and payload equals message bytes
	reqs, err := c.Sighashes()
	require.NoError(t, err)

	require.Len(t, reqs, 1, "expected 1 signature request")
	require.Equal(t, xc.Address(k1.String()), reqs[0].Signer, "unexpected signer")
	payload, _ := c.SolTx.Message.MarshalBinary()
	require.Equal(t, payload, reqs[0].Payload, "payload mismatch")

	// Set signature for the first signer index and validate Hash
	signers := c.SolTx.Message.Signers()
	i0 := 0 // first signature determines Hash()
	// ensure we set the signature for index 0
	pk0 := signers[i0]
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = 0xAB
	}
	require.NoError(t, c.SetSignatures(&xc.SignatureResponse{Signature: sig, PublicKey: pk0.Bytes()}), "SetSignatures failed")
	expectedHash := solana.Signature(sig).String()
	require.Equal(t, expectedHash, string(c.Hash()))
}

func TestNewCall_ErrsWhenAccountNotSigner(t *testing.T) {
	k1 := solana.NewWallet().PublicKey()
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, k1, k2)
	msgBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	// pick an unrelated account as the requested Account
	bad := solana.NewWallet().PublicKey()
	raw := mustMarshalCall(t, call.Call{Transaction: msgBytes})

	_, err = call.NewCall(&xc.ChainBaseConfig{}, xccall.SolanaSignTransaction, raw, xc.Address(bad.String()))
	require.Error(t, err)
}

func TestSolanaSetInput_NilAccepted(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	// use memo instructions as in existing tests
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, payer, k2)
	msgBytes, err := tx.MarshalBinary()
	require.NoError(t, err)
	raw := mustMarshalCall(t, call.Call{Transaction: msgBytes})
	c, err := call.NewCall(&xc.ChainBaseConfig{}, xccall.SolanaSignTransaction, raw, xc.Address(payer.String()))
	require.NoError(t, err)
	require.NoError(t, c.SetInput(nil), "SetInput(nil) should not error")
}

func TestSolanaNewCall_ContractAddresses_NoDuplicates(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	customProgram := solana.NewWallet().PublicKey() // unlikely to be a native program id
	tx := newCustomProgramTx(t, payer, customProgram)
	msgBytes, err := tx.MarshalBinary()
	require.NoError(t, err)

	raw := mustMarshalCall(t, call.Call{Transaction: msgBytes})
	c, err := call.NewCall(&xc.ChainBaseConfig{}, xccall.SolanaSignTransaction, raw, xc.Address(payer.String()))
	require.NoError(t, err)

	addrs := c.ContractAddresses()
	require.Len(t, addrs, 1, "expected exactly 1 contract address")
	require.Equal(t, customProgram.String(), string(addrs[0]), "unexpected contract address")
}
