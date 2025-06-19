package tx_input

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

// EOS doesn't seem to have a consistent way to represent an asset with a single ID.
// This is because two identifiers are needed:
// - The contract address (or "account" of the asset), e.g. "eosio.token"
// - The asset symbol, e.g. "EOS"
// I've found some consistency in these three examples:
// - https://www.unicove.com/en/vaulta/token/core.vaulta/A
// - https://eosflare.io/token/token.pcash/USDCASH
// - https://coinmarketcap.com/currencies/vaulta/
//   - (note CMC is using `core.vaulta` as the contract ID)
//
// The consistent pattern is `<contract_id>/<asset_symbol>`, where `<asset_symbol>` may be optional.
// The transaction input will need to contain all of the asset symbols for a given contract in case <asset_symbol> is not present.
func ParseContractId(chain *xc.ChainBaseConfig, contractId xc.ContractAddress, inputMaybe *TxInput) (contract string, symbol string, err error) {
	parts := splitByAny(string(contractId), []string{"/", "-"})
	if len(parts) == 1 {
		// Use the symbol from the input if it's present, meaning the RPC connector has resolved what the symbol should be.
		if inputMaybe != nil {
			if inputMaybe.Symbol != "" {
				return parts[0], inputMaybe.Symbol, nil
			}
		}
		// try to lookup the contract ID in the chain config
		// Map assetID to contractID/symbol
		for _, na := range chain.NativeAssets {
			if na.HasAlias(string(contractId)) {
				parts = splitByAny(string(na.ContractId), []string{"/", "-"})
				return parts[0], parts[1], nil
			}
		}
		return parts[0], "", fmt.Errorf(
			"unable to resolve symbol from contract, please ensure contract is formatted like `<contract>/<symbol>` (got '%s')",
			contractId,
		)

	}
	return parts[0], parts[1], nil
}

func splitByAny(s string, seps []string) []string {
	for _, sep := range seps {
		parts := strings.Split(s, sep)
		if len(parts) > 1 {
			return parts
		}
	}
	return []string{s}
}

// chains/EOS/assets/EOS
// - eosio.token/EOS
// - eosio.token

// chains/EOS/assets/core.vaulta/A
// - core.vaulta/A
// - core.vaulta

func DefaultContractId(chain *xc.ChainBaseConfig) xc.ContractAddress {
	chainCoin := chain.ChainCoin
	if chainCoin != "" {
		return xc.ContractAddress(chainCoin)
	}
	return "eosio.token/EOS"
}
