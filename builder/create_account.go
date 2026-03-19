package builder

import xc "github.com/cordialsys/crosschain"

type CreateAccountArgs struct {
	appliedOptions []BuilderOption
	options        builderOptions
	chain          xc.NativeAsset
	address        xc.Address
	publicKey      []byte
}

var _ TransactionOptions = &CreateAccountArgs{}

func NewCreateAccountArgs(chain xc.NativeAsset, address xc.Address, publicKey []byte, options ...BuilderOption) (CreateAccountArgs, error) {
	builderOptions := newBuilderOptions()
	args := CreateAccountArgs{
		appliedOptions: options,
		options:        builderOptions,
		chain:          chain,
		address:        address,
		publicKey:      append([]byte(nil), publicKey...),
	}
	for _, opt := range options {
		if err := opt(&args.options); err != nil {
			return args, err
		}
	}
	return args, nil
}

func (args *CreateAccountArgs) GetChain() xc.NativeAsset { return args.chain }
func (args *CreateAccountArgs) GetAddress() xc.Address   { return args.address }
func (args *CreateAccountArgs) PublicKeyBytes() []byte   { return append([]byte(nil), args.publicKey...) }
func (args *CreateAccountArgs) GetMemo() (string, bool)  { return args.options.GetMemo() }
func (args *CreateAccountArgs) GetTimestamp() (int64, bool) {
	return args.options.GetTimestamp()
}
func (args *CreateAccountArgs) GetPriority() (xc.GasFeePriority, bool) {
	return args.options.GetPriority()
}
func (args *CreateAccountArgs) GetPublicKey() ([]byte, bool) {
	return append([]byte(nil), args.publicKey...), len(args.publicKey) > 0
}
