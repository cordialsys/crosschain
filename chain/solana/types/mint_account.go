package types

import (
	xc "github.com/cordialsys/crosschain"
)

// This is the value.account.data for a Mint/Token Contract, e.g. https://solscan.io/token/HZ1JovNiVvGrGNiiYvEozEVgZ58xaU3RKwX8eACQBCt3
type MintAccountInfo struct {
	Parsed  MintAccountInfoParsed `json:"parsed"`
	Program string                `json:"program"`
	Space   uint64                `json:"space"`
}
type MintAccountInfoParsed struct {
	Info MintAccountInfoParsedInfo `json:"info"`
	Type string                    `json:"type"`
}
type MintAccountInfoParsedInfo struct {
	IsInitialized bool                `json:"isInitialized"`
	Decimals      uint64              `json:"decimals"`
	Supply        xc.AmountBlockchain `json:"supply"`
	Type          string              `json:"type"`
}
