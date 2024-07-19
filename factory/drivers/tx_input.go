package drivers

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/aptos"
	bitcointxinput "github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	evm_legacy "github.com/cordialsys/crosschain/chain/evm_legacy"
	"github.com/cordialsys/crosschain/chain/solana"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/cordialsys/crosschain/chain/tron"
)

func MarshalTxInput(txInput xc.TxInput) ([]byte, error) {
	return json.Marshal(txInput)
}

func NewTxInput(driver xc.Driver) (xc.TxInput, error) {
	switch driver {
	case xc.DriverAptos:
		return aptos.NewTxInput(), nil
	case xc.DriverCosmos, xc.DriverCosmosEvmos:
		return cosmos.NewTxInput(), nil
	case xc.DriverEVM:
		return evminput.NewTxInput(), nil
	case xc.DriverEVMLegacy:
		return evm_legacy.NewTxInput(), nil
	case xc.DriverSolana:
		return solana.NewTxInput(), nil
	case xc.DriverBitcoin, xc.DriverBitcoinCash, xc.DriverBitcoinLegacy:
		return bitcointxinput.NewTxInput(), nil
	case xc.DriverSui:
		return sui.NewTxInput(), nil
	case xc.DriverSubstrate:
		return substrate.NewTxInput(), nil
	case xc.DriverTron:
		return tron.NewTxInput(), nil
	case xc.DriverTon:
		return ton.NewTxInput(), nil
	}
	return nil, fmt.Errorf("no tx-input mapped for driver %s", driver)
}

func UnmarshalTxInput(data []byte) (xc.TxInput, error) {
	var env xc.TxInputEnvelope
	buf := []byte(data)
	err := json.Unmarshal(buf, &env)
	if err != nil {
		return nil, err
	}
	input, err := NewTxInput(env.Type)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(buf, input)
	if err != nil {
		return nil, err
	}
	return input, nil
}
