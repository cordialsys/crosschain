package drivers

import (
	"encoding/json"
	"fmt"
	"reflect"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const SerializedInputTypeKey = "type"

func MarshalTxInput(methodInput xc.TxInput) ([]byte, error) {
	data := map[string]interface{}{}
	methodBz, err := json.Marshal(methodInput)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(methodBz, &data)
	// force union with method type envelope
	if variant, ok := methodInput.(xc.TxVariantInput); ok {
		data[SerializedInputTypeKey] = variant.GetVariant()
	} else {
		data[SerializedInputTypeKey] = methodInput.GetDriver()
	}

	bz, _ := json.Marshal(data)
	return bz, nil
}

// Create a copy of a interface object, to avoid modifying the original
// in the tx-input registry.
func makeCopy[T any](input T) T {
	srcVal := reflect.ValueOf(input)
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	newVal := reflect.New(srcVal.Type())

	return newVal.Interface().(T)
}

func NewTxInput(driver xc.Driver) (xc.TxInput, error) {
	for _, txInput := range registry.GetSupportedBaseTxInputs() {
		if txInput.GetDriver() == driver {
			return makeCopy(txInput), nil
		}
		// aliases for fork chains
		switch driver {
		case xc.DriverBitcoin, xc.DriverBitcoinCash, xc.DriverBitcoinLegacy, xc.DriverZcash:
			if txInput.GetDriver() == xc.DriverBitcoin {
				return makeCopy(txInput), nil
			}
		case xc.DriverCosmos, xc.DriverCosmosEvmos:
			if txInput.GetDriver() == xc.DriverCosmos {
				return makeCopy(txInput), nil
			}
		}
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
		input2, err2 := NewVariantInput(xc.TxVariantInputType(env.Type))
		if err2 != nil {
			return nil, err
		}
		input = input2
	}
	err = json.Unmarshal(buf, input)
	if err != nil {
		return nil, err
	}
	return input, nil
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

	for _, variant := range registry.GetSupportedTxVariants() {
		if variant.GetVariant() == variantType {
			return makeCopy(variant), nil
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

func UnmarshalMultiTransferInput(data []byte) (xc.MultiTransferInput, error) {
	inp, err := UnmarshalVariantInput(data)
	if err != nil {
		return nil, err
	}
	multiTransfer, ok := inp.(xc.MultiTransferInput)
	if !ok {
		return multiTransfer, fmt.Errorf("not a multi-transfer input: %T", inp)
	}
	return multiTransfer, nil
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

func UnmarshalWithdrawingInput(data []byte) (xc.WithdrawTxInput, error) {
	inp, err := UnmarshalVariantInput(data)
	if err != nil {
		return nil, err
	}
	staking, ok := inp.(xc.WithdrawTxInput)
	if !ok {
		return staking, fmt.Errorf("not an unstaking input: %T", inp)
	}
	return staking, nil
}

func UnmarshalCallInput(data []byte) (xc.CallTxInput, error) {
	inp, err := UnmarshalVariantInput(data)
	if err != nil {
		return nil, err
	}
	call, ok := inp.(xc.CallTxInput)
	if !ok {
		return call, fmt.Errorf("not a call input: %T", inp)
	}
	return call, nil
}
