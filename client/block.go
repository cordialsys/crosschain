package client

import xc "github.com/cordialsys/crosschain"

type BlockArgs struct {
	height   uint64
	contract xc.ContractAddress
}

func (args *BlockArgs) Height() (uint64, bool) {
	return args.height, args.height > 0
}
func AtHeight(height uint64) *BlockArgs {
	return &BlockArgs{
		height:   height,
		contract: "",
	}
}

func LatestHeight() *BlockArgs {
	return &BlockArgs{}
}

func (args *BlockArgs) Contract() (xc.ContractAddress, bool) {
	return args.contract, args.contract != ""
}

func (args *BlockArgs) SetContract(contract xc.ContractAddress) {
	args.contract = contract
}
