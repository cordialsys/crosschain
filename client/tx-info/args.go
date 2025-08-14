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
	signTime int64
	// Optional block height
	blockHeight *xc.AmountBlockchain
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
	return args.signTime, args.signTime != 0
}

func (args *Args) BlockHeight() (xc.AmountBlockchain, bool) {
	if args.blockHeight == nil {
		return xc.AmountBlockchain{}, false
	}
	return *args.blockHeight, true
}

func (args *Args) SetTxSignTime(tx_time int64) {
	args.signTime = tx_time
}

func (args *Args) SetBlockHeight(height xc.AmountBlockchain) {
	args.blockHeight = &height
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

func OptionBlockHeight(height xc.AmountBlockchain) Option {
	return func(args *Args) {
		args.SetBlockHeight(height)
	}
}
