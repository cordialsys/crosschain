package substrate

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAlphaContract parses an alpha token contract address.
// The contract is just the netuid as a string (e.g., "64").
// Returns the parsed netuid.
func ParseAlphaContract(contract string) (netuid uint16, err error) {
	contract = strings.TrimSpace(contract)
	if contract == "" {
		return 0, fmt.Errorf("invalid alpha contract: empty string")
	}
	netuidVal, err := strconv.ParseUint(contract, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid alpha contract, expected a subnet netuid (uint16): %s", contract)
	}
	return uint16(netuidVal), nil
}
