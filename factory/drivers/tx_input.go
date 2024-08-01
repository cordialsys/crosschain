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
	solanatxinput "github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/crosschain/chain/ton"
	"github.com/cordialsys/crosschain/chain/tron"
)

const SerializedInputTypeKey = "type"

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
		return solanatxinput.NewTxInput(), nil
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

var SupportedVariantTx = []xc.TxVariantInput{
	&evminput.BatchDepositInput{},
	&evminput.ExitRequestInput{},
}

func MarshalVariantInput(methodInput xc.TxVariantInput) ([]byte, error) {
	data := map[string]interface{}{}
	methodBz, err := json.Marshal(methodInput)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(methodBz, &data)
	// force union with method type envelope
	data[SerializedInputTypeKey] = methodInput.GetVariant()

	bz, _ := json.Marshal(data)
	return bz, nil
}

func NewVariantInput(variantType xc.TxVariantInputType) (xc.TxVariantInput, error) {
	if err := variantType.Validate(); err != nil {
		return nil, err
	}

	for _, variant := range []xc.TxVariantInput{
		&evminput.BatchDepositInput{},
		&evminput.ExitRequestInput{},
	} {
		if variant.GetVariant() == variantType {
			return variant, nil
		}
	}

	return nil, fmt.Errorf("no staking-input mapped for %s", variantType)
}

func UnmarshalVariantInput(data []byte) (xc.TxVariantInput, error) {
	type variantInputEnvelope struct {
		Type xc.TxVariantInputType `json:"type"`
	}
	var env variantInputEnvelope
	buf := []byte(data)
	err := json.Unmarshal(buf, &env)
	if err != nil {
		return nil, err
	}
	input, err := NewVariantInput(env.Type)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(buf, input)
	if err != nil {
		return nil, err
	}
	return input, nil
}

func UnmarshalStakingInput(data []byte) (xc.StakeTxInput, error) {
	inp, err := UnmarshalVariantInput(data)
	if err != nil {
		return nil, err
	}
	staking, ok := inp.(xc.StakeTxInput)
	if !ok {
		return staking, fmt.Errorf("not a staking input: %T", inp)
	}
	return staking, nil
}

func UnmarshalUnstakingInput(data []byte) (xc.UnstakeTxInput, error) {
	inp, err := UnmarshalVariantInput(data)
	if err != nil {
		return nil, err
	}
	staking, ok := inp.(xc.UnstakeTxInput)
	if !ok {
		return staking, fmt.Errorf("not an unstaking input: %T", inp)
	}
	return staking, nil
}
