package utils

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/normalize"
)

// Common implementation for capturing price information in a tx-input
type Price struct {
	Contract string                 `json:"contract"`
	Chain    xc.NativeAsset         `json:"chain"`
	PriceUsd xc.AmountHumanReadable `json:"price_usd"`
}

type TxPriceInput struct {
	Prices []*Price `json:"prices,omitempty"`
}

var _ xc.TxInputWithPricing = &TxPriceInput{}

func (input *TxPriceInput) SetUsdPrice(chain xc.NativeAsset, contract string, priceUsd xc.AmountHumanReadable) {
	// normalize the contract
	contract = normalize.Normalize(contract, string(chain))
	// remove any existing
	input.removeUsdPrice(chain, contract)
	// add new
	input.Prices = append(input.Prices, &Price{
		Contract: contract,
		Chain:    chain,
		PriceUsd: priceUsd,
	})
}

func (input *TxPriceInput) GetUsdPrice(chain xc.NativeAsset, contract string) (xc.AmountHumanReadable, bool) {
	contract = normalize.Normalize(contract, string(chain))
	for _, price := range input.Prices {
		if price.Chain == chain && price.Contract == contract {
			return price.PriceUsd, true
		}
	}
	return xc.AmountHumanReadable{}, false
}

func (input *TxPriceInput) removeUsdPrice(chain xc.NativeAsset, contract string) {
	for i, price := range input.Prices {
		if price.Chain == chain && price.Contract == contract {
			input.Prices = append(input.Prices[:i], input.Prices[i+1:]...)
			return
		}
	}
}
