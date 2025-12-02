package call

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/memo"
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
	if err != nil {
		t.Fatalf("failed to build tx: %v", err)
	}
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
	if err != nil {
		t.Fatalf("failed to build tx: %v", err)
	}
	return tx
}

func mustMarshalCall(t *testing.T, call Call) json.RawMessage {
	b, err := json.Marshal(call)
	if err != nil {
		t.Fatalf("failed to marshal call: %v", err)
	}
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
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}

	// Build Call JSON targeting k1 as the requested account
	raw := mustMarshalCall(t, Call{Transaction: msgBytes, Account: k1})
	cfg := &xc.ChainBaseConfig{}
	c, err := NewCall(cfg, raw)
	if err != nil {
		t.Fatalf("NewCall failed: %v", err)
	}

	// Map indices for k1 and k2
	signers := c.solTx.Message.Signers()
	i1 := findSignerIndex(signers, k1)
	i2 := findSignerIndex(signers, k2)
	if i1 < 0 || i2 < 0 {
		t.Fatalf("expected both k1 and k2 to be signers, got: %v", signers)
	}

	// First, set signature for k1 only
	sig1 := make([]byte, 64)
	for i := range sig1 {
		sig1[i] = 0x11
	}
	err = c.SetSignatures(&xc.SignatureResponse{Signature: sig1, PublicKey: k1.Bytes()})
	if err != nil {
		t.Fatalf("SetSignatures failed: %v", err)
	}
	if len(c.solTx.Signatures) != len(signers) {
		t.Fatalf("expected %d signatures, got %d", len(signers), len(c.solTx.Signatures))
	}
	if c.solTx.Signatures[i1].String() == (solana.Signature{}).String() {
		t.Fatalf("expected signature for k1 to be set")
	}
	if c.solTx.Signatures[i2].String() != (solana.Signature{}).String() {
		t.Fatalf("expected signature for k2 to be empty initially")
	}

	// Now, set signature for k2 only; k1 should be preserved
	sig2 := make([]byte, 64)
	for i := range sig2 {
		sig2[i] = 0x22
	}
	err = c.SetSignatures(&xc.SignatureResponse{Signature: sig2, PublicKey: k2.Bytes()})
	if err != nil {
		t.Fatalf("SetSignatures failed: %v", err)
	}
	if c.solTx.Signatures[i1].String() == (solana.Signature{}).String() {
		t.Fatalf("expected signature for k1 to be preserved, but it was cleared")
	}
	if c.solTx.Signatures[i2].String() == (solana.Signature{}).String() {
		t.Fatalf("expected signature for k2 to be set, but it is empty")
	}

	// Calling with empty slice should not wipe existing signatures
	err = c.SetSignatures()
	if err != nil {
		t.Fatalf("SetSignatures(empty) failed: %v", err)
	}
	if c.solTx.Signatures[i1].String() == (solana.Signature{}).String() || c.solTx.Signatures[i2].String() == (solana.Signature{}).String() {
		t.Fatalf("expected existing signatures to be preserved when passing no signatures")
	}
}

func TestSighashesAndHashBehavior(t *testing.T) {
	k1 := solana.NewWallet().PublicKey()
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, k1, k2)
	msgBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}

	raw := mustMarshalCall(t, Call{Transaction: msgBytes, Account: k1})
	c, err := NewCall(&xc.ChainBaseConfig{}, raw)
	if err != nil {
		t.Fatalf("NewCall failed: %v", err)
	}

	// Hash should be empty initially
	if got := c.Hash(); got != "" {
		t.Fatalf("expected empty hash, got %q", got)
	}

	// Sighashes should target k1 and payload equals message bytes
	reqs, err := c.Sighashes()
	if err != nil {
		t.Fatalf("Sighashes failed: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 signature request, got %d", len(reqs))
	}
	if reqs[0].Signer != xc.Address(k1.String()) {
		t.Fatalf("expected signer %s, got %s", k1.String(), reqs[0].Signer)
	}
	payload, _ := c.solTx.Message.MarshalBinary()
	if string(reqs[0].Payload) != string(payload) {
		t.Fatalf("payload mismatch")
	}

	// Set signature for the first signer index and validate Hash
	signers := c.solTx.Message.Signers()
	i0 := 0 // first signature determines Hash()
	// ensure we set the signature for index 0
	pk0 := signers[i0]
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = 0xAB
	}
	if err := c.SetSignatures(&xc.SignatureResponse{Signature: sig, PublicKey: pk0.Bytes()}); err != nil {
		t.Fatalf("SetSignatures failed: %v", err)
	}
	expectedHash := solana.Signature(sig).String()
	if got := c.Hash(); string(got) != expectedHash {
		t.Fatalf("expected hash %s, got %s", expectedHash, got)
	}
}

func TestNewCall_ErrsWhenAccountNotSigner(t *testing.T) {
	k1 := solana.NewWallet().PublicKey()
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, k1, k2)
	msgBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}

	// pick an unrelated account as the requested Account
	bad := solana.NewWallet().PublicKey()
	raw := mustMarshalCall(t, Call{Transaction: msgBytes, Account: bad})
	_, err = NewCall(&xc.ChainBaseConfig{}, raw)
	if err == nil {
		t.Fatalf("expected NewCall to error when Account is not among message signers")
	}
}

func TestSolanaSetInput_NilAccepted(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	// use memo instructions as in existing tests
	k2 := solana.NewWallet().PublicKey()
	tx := newTwoSignerTx(t, payer, k2)
	msgBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}
	raw := mustMarshalCall(t, Call{Transaction: msgBytes, Account: payer})
	c, err := NewCall(&xc.ChainBaseConfig{}, raw)
	if err != nil {
		t.Fatalf("NewCall failed: %v", err)
	}
	if err := c.SetInput(nil); err != nil {
		t.Fatalf("SetInput(nil) should not error, got: %v", err)
	}
}

func TestSolanaNewCall_ContractAddresses_NoDuplicates(t *testing.T) {
	payer := solana.NewWallet().PublicKey()
	customProgram := solana.NewWallet().PublicKey() // unlikely to be a native program id
	tx := newCustomProgramTx(t, payer, customProgram)
	msgBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}
	raw := mustMarshalCall(t, Call{Transaction: msgBytes, Account: payer})
	c, err := NewCall(&xc.ChainBaseConfig{}, raw)
	if err != nil {
		t.Fatalf("NewCall failed: %v", err)
	}
	addrs := c.ContractAddresses()
	if len(addrs) != 1 {
		t.Fatalf("expected exactly 1 contract address, got %d: %v", len(addrs), addrs)
	}
	if string(addrs[0]) != customProgram.String() {
		t.Fatalf("unexpected contract address: want %s got %s", customProgram.String(), addrs[0])
	}
}
