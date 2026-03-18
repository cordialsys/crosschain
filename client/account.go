package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

// CreateAccountInput holds the data required to create (register) an on-chain account.
// It may contain one or more payloads that must be signed by the party's private key
// before calling CreateAccount.
//
// The flow mirrors the Canton external-party registration process:
//  1. Call FetchCreateAccountInput to obtain the input and the hashes to sign.
//  2. Sign each hash returned by Sighashes().
//  3. Call CreateAccount with the signed input.
type CreateAccountInput interface {
	// Sighashes returns the ordered list of payloads the party must sign.
	// Each entry corresponds to one signature that must be passed to SetSignatures.
	Sighashes() ([]*xc.SignatureRequest, error)

	// SetSignatures attaches the signatures produced by the external signer.
	// Signatures must be provided in the same order as Sighashes().
	SetSignatures(sigs ...*xc.SignatureResponse) error

	// VerifySignaturePayloads recomputes the expected hash(es) from the raw
	// registration data stored in the input and checks they match what Sighashes()
	// returned.  This lets a caller confirm the input has not been tampered with
	// before signing.
	VerifySignaturePayloads() error

	// Serialize encodes the pending registration step so it can be signed and
	// later submitted through the generic submit path.
	Serialize() ([]byte, error)
}

// AccountClient is an optional client interface for chains that require an explicit
// on-chain account creation or registration step before the account can receive funds
// or submit transactions.
type AccountClient interface {
	// FetchCreateAccountInput advances all account-creation steps that do not need
	// an external signature. If a signature is needed for the next step, it returns
	// the corresponding CreateAccountInput; if registration is already complete, it
	// returns nil.
	FetchCreateAccountInput(ctx context.Context, args *CreateAccountArgs) (CreateAccountInput, error)

	// CreateAccount submits one previously signed registration step to the chain.
	CreateAccount(ctx context.Context, input CreateAccountInput) error
}

// CreateAccountArgs carries the parameters for an account creation request.
type CreateAccountArgs struct {
	// Address (party ID) of the account to register.
	address xc.Address
	// Raw public key bytes of the account owner.
	publicKey []byte
}

func NewCreateAccountArgs(address xc.Address, publicKey []byte) *CreateAccountArgs {
	return &CreateAccountArgs{
		address:   address,
		publicKey: publicKey,
	}
}

func (a *CreateAccountArgs) GetAddress() xc.Address { return a.address }
func (a *CreateAccountArgs) GetPublicKey() []byte   { return a.publicKey }
