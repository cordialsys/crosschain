package tx_input

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	"github.com/solana-foundation/solana-go/v2"
)

const (
	LamportsPerSignature           uint64 = 5000
	TransactionVersionLegacy              = "legacy"
	TransactionVersionV0                  = "v0"
	TransactionVersionV1                  = "v1"
	MaxComputeUnitLimit            uint32 = 1_400_000
	MaxLoadedAccountsDataSizeLimit uint32 = 64 * 1024 * 1024
)

func (input *TxInput) MessageVersion() (solana.MessageVersion, error) {
	switch input.TransactionVersion {
	case "", TransactionVersionLegacy:
		return solana.MessageVersionLegacy, nil
	case TransactionVersionV0:
		return solana.MessageVersionV0, nil
	case TransactionVersionV1:
		return solana.MessageVersionV1, nil
	default:
		return 0, fmt.Errorf("unsupported Solana transaction version %q", input.TransactionVersion)
	}
}

func (input *TxInput) V1ComputeUnitLimit() uint32 {
	if input.TransactionConfig != nil {
		if input.TransactionConfig.ComputeUnitLimit != nil {
			return *input.TransactionConfig.ComputeUnitLimit
		}
		return MaxComputeUnitLimit
	}
	if input.ComputeUnitLimit == 0 {
		return MaxComputeUnitLimit
	}
	return input.ComputeUnitLimit
}

// V1PriorityFee uses an explicit config's absolute fee when present.
// Otherwise it converts the existing RPC price (micro-lamports/CU) to a total
// lamport fee, rounding up. Keep the price in TxInput so priority multipliers
// and existing fee estimation APIs retain their units.
func (input *TxInput) V1PriorityFee() xc.AmountBlockchain {
	if input.TransactionConfig != nil {
		if input.TransactionConfig.PriorityFee != nil {
			return xc.NewAmountBlockchainFromUint64(*input.TransactionConfig.PriorityFee)
		}
		return xc.NewAmountBlockchainFromUint64(0)
	}
	fee := new(big.Int).Mul(input.PrioritizationFee.Int(), new(big.Int).SetUint64(uint64(input.V1ComputeUnitLimit())))
	fee.Add(fee, big.NewInt(999_999))
	fee.Div(fee, big.NewInt(1_000_000))
	return xc.AmountBlockchain(*fee)
}

func (input *TxInput) V1Config() (solana.TransactionConfig, error) {
	if input.TransactionConfig != nil {
		config := *input.TransactionConfig
		if config.ComputeUnitLimit != nil && *config.ComputeUnitLimit > MaxComputeUnitLimit ||
			config.LoadedAccountsDataSizeLimit != nil && *config.LoadedAccountsDataSizeLimit > MaxLoadedAccountsDataSizeLimit {
			return solana.TransactionConfig{}, fmt.Errorf("Solana v1 resource limits exceed runtime maximum")
		}
		return config, nil
	}
	units := input.V1ComputeUnitLimit()
	dataSize := input.LoadedAccountsDataSizeLimit
	if dataSize == 0 {
		dataSize = MaxLoadedAccountsDataSizeLimit
	}
	if units > MaxComputeUnitLimit || dataSize > MaxLoadedAccountsDataSizeLimit {
		return solana.TransactionConfig{}, fmt.Errorf("Solana v1 resource limits exceed runtime maximum")
	}
	fee := input.V1PriorityFee()
	if input.PrioritizationFee.Int().Sign() < 0 || !fee.Int().IsUint64() {
		return solana.TransactionConfig{}, fmt.Errorf("Solana v1 priority fee is outside the uint64 lamport range")
	}
	return solana.TransactionConfig{}.
		WithComputeUnitLimit(units).
		WithLoadedAccountsDataSizeLimit(dataSize).
		WithPriorityFee(fee.Uint64()), nil
}
