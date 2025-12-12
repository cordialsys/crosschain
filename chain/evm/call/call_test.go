package call_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/call"
)

type wrongCallInput struct{}

// Implement xc.TxInput
func (wrongCallInput) GetDriver() xc.Driver                      { return xc.DriverEVM }
func (wrongCallInput) IndependentOf(xc.TxInput) bool             { return true }
func (wrongCallInput) SafeFromDoubleSend(xc.TxInput) bool        { return true }
func (wrongCallInput) SetGasFeePriority(xc.GasFeePriority) error { return nil }
func (wrongCallInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return xc.NewAmountBlockchainFromUint64(0), ""
}

// Implement xc.TxVariantInput
func (wrongCallInput) GetVariant() xc.TxVariantInputType { return xc.NewCallingInputType(xc.DriverEVM) }

// Implement xc.CallTxInput
func (wrongCallInput) Calling() {}

func TestNewCall(t *testing.T) {
	vectors := []struct {
		params []call.Params
		result error
	}{
		{
			params: nil,
			result: fmt.Errorf("only params with a signle element supported for now, got 0"),
		},
		{
			params: []call.Params{},
			result: fmt.Errorf("only params with a signle element supported for now, got 0"),
		},
		{
			params: []call.Params{
				{
					From: "0x1234",
					To:   "0x5678",
				},
				{
					From: "0x1234",
					To:   "0x5678",
				},
			},
			result: fmt.Errorf("only params with a signle element supported for now, got 2"),
		},
		{
			params: []call.Params{
				{
					From: "0x1234",
					To:   "0x5678",
				},
			},
			result: nil,
		},
	}

	for i, v := range vectors {
		msgBytes, _ := json.Marshal(call.Call{Method: "eth_call", Params: v.params})
		_, err := call.NewCall(&xc.ChainBaseConfig{}, msgBytes)

		switch {
		case err == nil && v.result != nil:
			t.Fatalf("testcase %d: expected error `%v`, got nil", i, v.result)
		case err != nil && v.result == nil:
			t.Fatalf("testcase %d: expected no error, got `%v`", i, err)
		case err != nil && v.result != nil && err.Error() != v.result.Error():
			t.Fatalf("testcase %d: expected `%v`, got `%v`", i, v.result, err)
		}
	}
}

func TestSetInput(t *testing.T) {
	vectors := []struct {
		input  xc.CallTxInput
		result error
	}{
		{
			input:  nil,
			result: fmt.Errorf("input not set"),
		},
		{
			input:  wrongCallInput{},
			result: fmt.Errorf("expected input type *tx_input.CallInput, got %T", wrongCallInput{}),
		},
	}
	for i, v := range vectors {
		msgBytes, _ := json.Marshal(call.Call{Method: "eth_call", Params: []call.Params{
			{
				From: "0x1234",
				To:   "0x5678",
			},
		}})
		c, err := call.NewCall(&xc.ChainBaseConfig{}, msgBytes)
		if err != nil {
			t.Fatalf("NewCall failed: %v", err)
		}
		err = c.SetInput(v.input)
		switch {
		case err == nil && v.result != nil:
			t.Fatalf("testcase %d: expected error `%v`, got nil", i, v.result)
		case err != nil && v.result == nil:
			t.Fatalf("testcase %d: expected no error, got `%v`", i, err)
		case err != nil && v.result != nil && err.Error() != v.result.Error():
			t.Fatalf("testcase %d: expected `%v`, got `%v`", i, v.result, err)
		}
	}
}
