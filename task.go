package crosschain

type ITask interface {
	GetChain() *ChainConfig
	// Get associated asset decimals if it exists
	GetDecimals() int32
	// Get associated contract if it exists
	GetContract() string

	// Informational / debugging
	String() string
	// Get asset symbol (e.g. 'USDC') if it exists.  Does not include the chain.  Informational.
	GetAssetSymbol() string
}
