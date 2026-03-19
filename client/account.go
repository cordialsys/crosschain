package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

// CreateAccountInputClient is an optional client interface for chains that require
// an explicit on-chain account creation or registration step before the account can
// receive funds or submit transactions.
type CreateAccountInputClient interface {
	// FetchCreateAccountInput advances all account-creation steps that do not need
	// an external signature. If a signature is needed for the next step, it returns
	// the corresponding tx input; if registration is already complete, it
	// returns nil.
	FetchCreateAccountInput(ctx context.Context, args *CreateAccountArgs) (xc.CreateAccountTxInput, error)
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
