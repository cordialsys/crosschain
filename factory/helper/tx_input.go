package helper

import (
	"encoding/json"
	"fmt"

	xc "github.com/jumpcrypto/crosschain"
	"github.com/jumpcrypto/crosschain/chain/aptos"
	"github.com/jumpcrypto/crosschain/chain/bitcoin"
	"github.com/jumpcrypto/crosschain/chain/cosmos"
	"github.com/jumpcrypto/crosschain/chain/evm"
	"github.com/jumpcrypto/crosschain/chain/solana"
	"github.com/jumpcrypto/crosschain/chain/substrate"
	"github.com/jumpcrypto/crosschain/chain/sui"
)

func MarshalTxInput(txInput xc.TxInput) ([]byte, error) {
	return json.Marshal(txInput)
}

func UnmarshalTxInput(data []byte) (xc.TxInput, error) {
	var env xc.TxInputEnvelope
	buf := []byte(data)
	err := json.Unmarshal(buf, &env)
	if err != nil {
		return nil, err
	}
	switch env.Type {
	case xc.DriverAptos:
		var txInput aptos.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		var txInput cosmos.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverEVM, xc.DriverEVMLegacy:
		var txInput evm.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverSolana:
		var txInput solana.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverBitcoin:
		var txInput bitcoin.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverSui:
		var txInput sui.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	case xc.DriverSubstrate:
		var txInput substrate.TxInput
		err := json.Unmarshal(buf, &txInput)
		return &txInput, err
	default:
		return nil, fmt.Errorf("invalid TxInput type: %s", env.Type)
	}
}
