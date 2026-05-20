package client

import (
	"time"

	xc "github.com/cordialsys/crosschain"
)

type OfferArgs struct {
	address  xc.Address
	contract xc.ContractAddress
}

func NewOfferArgs(address xc.Address, options ...GetOfferOption) *OfferArgs {
	args := &OfferArgs{address: address}
	for _, option := range options {
		option(args)
	}
	return args
}

func (args *OfferArgs) Address() xc.Address {
	return args.address
}

func (args *OfferArgs) Contract() (xc.ContractAddress, bool) {
	return args.contract, args.contract != ""
}

type GetOfferOption func(*OfferArgs)

func OfferOptionContract(contract xc.ContractAddress) GetOfferOption {
	return func(args *OfferArgs) {
		args.contract = contract
	}
}

type Offer struct {
	ID         string              `json:"id"`
	AssetID    xc.ContractAddress  `json:"asset_id"`
	From       xc.Address          `json:"from"`
	To         xc.Address          `json:"to"`
	Amount     xc.AmountBlockchain `json:"amount"`
	ExpiresAt  *time.Time          `json:"expires_at,omitempty"`
	TrackingID string              `json:"tracking_id,omitempty"`
}

type Settlement struct {
	ID         string              `json:"id"`
	AssetID    xc.ContractAddress  `json:"asset_id"`
	From       xc.Address          `json:"from"`
	To         xc.Address          `json:"to"`
	Amount     xc.AmountBlockchain `json:"amount"`
	ExpiresAt  *time.Time          `json:"expires_at,omitempty"`
	TrackingID string              `json:"tracking_id,omitempty"`
}
