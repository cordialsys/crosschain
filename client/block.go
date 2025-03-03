package client

type BlockArgs struct {
	height uint64
}

func (args *BlockArgs) Height() (uint64, bool) {
	return args.height, args.height > 0
}
func AtHeight(height uint64) *BlockArgs {
	return &BlockArgs{height}
}
func LatestHeight() *BlockArgs {
	return &BlockArgs{}
}
