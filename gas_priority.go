package crosschain

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

type GasFeePriority string

var Low GasFeePriority = "low"
var Market GasFeePriority = "market"
var Aggressive GasFeePriority = "aggressive"
var VeryAggressive GasFeePriority = "very-aggressive"

func NewPriority(input string) (GasFeePriority, error) {
	p := GasFeePriority(input)
	if p.IsEnum() {
		return p, nil
	}
	_, err := p.AsCustom()
	return p, err
}

func (p GasFeePriority) IsEnum() bool {
	switch p {
	case Low, Market, Aggressive, VeryAggressive:
		return true
	}
	return false
}

func (p GasFeePriority) AsCustom() (decimal.Decimal, error) {
	if p.IsEnum() {
		return decimal.Decimal{}, errors.New("not a custom enum")
	}
	dec, err := decimal.NewFromString(string(p))
	if err != nil {
		return dec, fmt.Errorf("invalid decimal: %v", err)
	}

	return dec, nil
}

func (p GasFeePriority) GetDefault() (decimal.Decimal, error) {
	switch p {
	case Low:
		return decimal.NewFromFloat(0.7), nil
	case Market:
		// use int for market to be exact 1
		return decimal.NewFromInt(1), nil
	case Aggressive:
		return decimal.NewFromFloat(1.5), nil
	case VeryAggressive:
		return decimal.NewFromInt(2), nil
	}
	return p.AsCustom()
}

// Check against the max-fee defaults in the configuration.
// Custody products should have a way to override max-fee limits.
func CheckMaxFeeLimit(input TxInput, chainConfig *ChainConfig) error {
	// Protect against fee griefing
	maxFeeSpend, feeAssetId := input.GetFeeLimit()
	if feeAssetId == "" || feeAssetId == ContractAddress(chainConfig.Chain) {
		maxFeeLimit := chainConfig.MaxFee.ToBlockchain(chainConfig.Decimals)
		if maxFeeSpend.Cmp(&maxFeeLimit) > 0 {
			maxFeeSpendHuman := maxFeeSpend.ToHuman(chainConfig.Decimals)
			return fmt.Errorf(
				"transaction fee may cost up to %s %s, which is greater than the current limit of %s",
				maxFeeSpendHuman.String(),
				chainConfig.Chain,
				chainConfig.MaxFee.String(),
			)
		}
	} else {
		var additionalAsset *AdditionalNativeAsset
		for _, asset := range chainConfig.AdditionalNativeAssets {
			if asset.AssetId == feeAssetId {
				additionalAsset = asset
				break
			}
		}
		if additionalAsset == nil {
			return fmt.Errorf("fee is in asset '%s', but there is no max-limit configured for this asset", feeAssetId)
		}
		maxFeeLimit := additionalAsset.MaxFee.ToBlockchain(additionalAsset.Decimals)
		maxFeeSpendHuman := maxFeeSpend.ToHuman(additionalAsset.Decimals)
		if maxFeeSpend.Cmp(&maxFeeLimit) > 0 {
			return fmt.Errorf(
				"transaction fee may cost up to %s %s, which is greater than the current limit of %s",
				maxFeeSpendHuman.String(),
				additionalAsset.AssetId,
				additionalAsset.MaxFee.String(),
			)
		}
	}
	return nil
}
