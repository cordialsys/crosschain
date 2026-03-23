package client

import (
	"context"

	xc "github.com/cordialsys/crosschain"
)

// CreateAccountClient is an optional client interface for chains that require
// an explicit on-chain account creation or registration step before the account can
// receive funds or submit transactions.
type CreateAccountClient interface {
	// FetchCreateAccountInput advances all account-creation steps that do not need
	// an external signature. If a signature is needed for the next step, it returns
	// the corresponding tx input; if registration is already complete, it
	// returns nil.
	FetchCreateAccountInput(ctx context.Context, args *CreateAccountArgs) (xc.CreateAccountTxInput, error)

	// GetAccountState returns the current create-account state without mutating it.
	GetAccountState(ctx context.Context, args *CreateAccountArgs) (*AccountState, error)
}

type AccountStateEnum string

const (
	CreateAccountCallRequired AccountStateEnum = "CreateAccountCallRequired"
	Pending                   AccountStateEnum = "Pending"
	Created                   AccountStateEnum = "Created"
)

type AccountState struct {
	State       AccountStateEnum `json:"state"`
	Description string           `json:"description"`
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
