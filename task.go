package crosschain

type ITask interface {
	GetChain() *ChainConfig
	GetDecimals() int32

	// Informational / debugging
	String() string
}
