package crosschain

type ITask interface {
	// unique identifier used within crosschain, typically a combination of asset.chain
	ID() AssetID
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
