package contract

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

// ExtractAssetAndContract parse assetContract and returns asset and contract
func NewContract(symbol string, issuerAddress string) (contract xc.ContractAddress) {
	return xc.ContractAddress(fmt.Sprintf("%s-%s", symbol, issuerAddress))
}

func ExtractAssetAndContract(assetContract string) (asset string, contract string, err error) {
	var separator string

	switch {
	case strings.Contains(assetContract, "."):
		separator = "."
	case strings.Contains(assetContract, "-"):
		separator = "-"
	case strings.Contains(assetContract, "_"):
		separator = "_"
	default:
		return "", "", fmt.Errorf("string must contain one of the following separators: '.', '-', '_'")
	}

	parts := strings.Split(assetContract, separator)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format, string should contain exactly one separator")
	}

	asset = parts[0]
	contract = parts[1]

	return asset, contract, nil
}
