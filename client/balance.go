package client

import xc "github.com/cordialsys/crosschain"

type BalanceArgs struct {
	address  xc.Address
	contract xc.ContractAddress
}

func (args *BalanceArgs) Address() xc.Address {
	return args.address
}

func (args *BalanceArgs) Contract() (xc.ContractAddress, bool) {
	return args.contract, args.contract != ""
}

func (args *BalanceArgs) SetContract(contract xc.ContractAddress) {
	args.contract = contract
}

func NewBalanceArgs(address xc.Address, options ...GetBalanceOption) *BalanceArgs {
	args := &BalanceArgs{address: address}
	for _, option := range options {
		option(args)
	}
	return args
}

type GetBalanceOption func(*BalanceArgs)

func BalanceOptionContract(contract xc.ContractAddress) GetBalanceOption {
	return func(args *BalanceArgs) {
		args.contract = contract
	}
}
