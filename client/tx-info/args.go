package txinfo

import (
	xc "github.com/cordialsys/crosschain"
)

type Args struct {
	// Hash of the transaction
	hash xc.TxHash
	// Optional contract address
	contract xc.ContractAddress
	// Optional sender address
	sender xc.Address
	// Optional sign time
	sign_time int64
}

func (args *Args) TxHash() xc.TxHash {
	return args.hash
}

func (args *Args) SetHash(hash xc.TxHash) {
	args.hash = hash
}

func (args *Args) Contract() (xc.ContractAddress, bool) {
	return args.contract, args.contract != ""
}

func (args *Args) SetContract(contract xc.ContractAddress) {
	args.contract = contract
}

func (args *Args) Sender() (xc.Address, bool) {
	return args.sender, args.sender != ""
}

func (args *Args) SetSender(sender xc.Address) {
	args.sender = sender
}

func (args *Args) TxSignTime() (int64, bool) {
	return args.sign_time, args.sign_time != 0
}

func (args *Args) SetTxSignTime(tx_time int64) {
	args.sign_time = tx_time
}

func NewArgs(hash xc.TxHash, options ...Option) *Args {
	args := &Args{hash: hash}
	for _, option := range options {
		option(args)
	}
	return args
}

type Option func(*Args)

func OptionContract(contract xc.ContractAddress) Option {
	return func(args *Args) {
		args.SetContract(contract)
	}
}

func OptionSender(sender xc.Address) Option {
	return func(args *Args) {
		args.SetSender(sender)
	}
}

func OptionSignTime(tx_time int64) Option {
	return func(args *Args) {
		args.SetTxSignTime(tx_time)
	}
}
