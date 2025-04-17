package client

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

func NewUtxoId(txHash xc.TxHash, vout int) string {
	// "txid:vout" is how mempool + other popular BTC programs identify UTXOs
	return fmt.Sprintf("%s:%d", txHash, vout)
}
