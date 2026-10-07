package tx_input

import xc "github.com/cordialsys/crosschain"

const UsdcPerps = "USDCPerps"
const UsdcDecimals = 8

// IsPerpsContract recognizes perps USDC and its legacy empty-contract form.
// HYPE and spot USDC use their own spot token contracts.
func IsPerpsContract(contract xc.ContractAddress) bool {
	return contract == "" || contract == UsdcPerps
}
