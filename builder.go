package crosschain

// TxBuilder is a Builder that can transfer assets
type TxBuilder interface {
	NewTransfer(from Address, to Address, amount AmountBlockchain, input TxInput) (Tx, error)
}

// TxTokenBuilder is a Builder that can transfer token assets, in addition to native assets
// This interface is soon being removed.
type TxTokenBuilder interface {
	TxBuilder
	NewNativeTransfer(from Address, to Address, amount AmountBlockchain, input TxInput) (Tx, error)
	NewTokenTransfer(from Address, to Address, amount AmountBlockchain, input TxInput) (Tx, error)
}

// TxXTransferBuilder is a Builder that can mutate an asset into another asset
// This interface is soon being removed.
type TxXTransferBuilder interface {
	TxBuilder
	NewTask(from Address, to Address, amount AmountBlockchain, input TxInput) (Tx, error)
}
