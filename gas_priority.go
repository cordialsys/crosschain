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
func CheckFeeLimit(input TxInput, chainConfig *ChainConfig) error {
	// Protect against fee griefing
	FeeLimitSpend, feeAssetId := input.GetFeeLimit()
	if feeAssetId == "" || feeAssetId == ContractAddress(chainConfig.Chain) {
		FeeLimitLimit := chainConfig.FeeLimit.ToBlockchain(chainConfig.Decimals)
		if FeeLimitSpend.Cmp(&FeeLimitLimit) > 0 {
			FeeLimitSpendHuman := FeeLimitSpend.ToHuman(chainConfig.Decimals)
			return fmt.Errorf(
				"transaction fee may cost up to %s %s, which is greater than the current limit of %s",
				FeeLimitSpendHuman.String(),
				chainConfig.Chain,
				chainConfig.FeeLimit.String(),
			)
		}
	} else {
		var additionalAsset *AdditionalNativeAsset
		for _, asset := range chainConfig.NativeAssets {
			if asset.ContractId == feeAssetId {
				additionalAsset = asset
				break
			}
		}
		if additionalAsset == nil {
			return fmt.Errorf("fee is in asset '%s', but there is no max-limit configured for this asset", feeAssetId)
		}
		FeeLimitLimit := additionalAsset.FeeLimit.ToBlockchain(additionalAsset.Decimals)
		FeeLimitSpendHuman := FeeLimitSpend.ToHuman(additionalAsset.Decimals)
		if FeeLimitSpend.Cmp(&FeeLimitLimit) > 0 {
			return fmt.Errorf(
				"transaction fee may cost up to %s %s, which is greater than the current limit of %s",
				FeeLimitSpendHuman.String(),
				additionalAsset.AssetId,
				additionalAsset.FeeLimit.String(),
			)
		}
	}
	return nil
}
