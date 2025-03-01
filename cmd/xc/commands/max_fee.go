package commands

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

func CheckMaxFeeLimit(input xc.TxInput, chainConfig *xc.ChainConfig) error {
	// Protect against fee griefing
	maxFeeSpend, feeAssetId := input.GetMaxFee()
	// Use the max-fee defaults in the configuration, but real custody product may define their own max fee's
	if feeAssetId == "" || feeAssetId == xc.ContractAddress(chainConfig.Chain) {
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
		var additionalAsset *xc.AdditionalNativeAsset
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
