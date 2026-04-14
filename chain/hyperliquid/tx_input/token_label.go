package tx_input

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
)

type TokenLabel string

func NewTokenLabel(symbol string, contract xc.ContractAddress) TokenLabel {
	if strings.Contains(string(contract), ":") {
		return TokenLabel(string(contract))
	}
	return TokenLabel(symbol + ":" + string(contract))
}

func (t TokenLabel) String() string {
	return string(t)
}

func (t TokenLabel) GetSymbol() (string, bool) {
	if strings.Contains(string(t), ":") {
		return strings.Split(string(t), ":")[0], true
	}
	return "", false
}

func (t TokenLabel) GetContract() (xc.ContractAddress, bool) {
	if strings.Contains(string(t), ":") {
		return xc.ContractAddress(strings.Split(string(t), ":")[1]), true
	}
	return "", false
}
