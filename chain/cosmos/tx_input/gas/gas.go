package gas

import (
	"errors"
	"math/big"
	"sort"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const NativeTransferGasLimit = uint64(400_000)
const TokenTransferGasLimit = uint64(900_000)

// Divide totalFee/totalGas and return as float safely
func TotalFeeToFeePerGas(totalFee string, totalGas uint64) float64 {
	ten := big.NewInt(10)
	precision := big.NewInt(10)
	precisionFactor := precision.Exp(ten, precision, nil)
	minFee, _ := new(big.Int).SetString(totalFee, 10)
	// avoid losing precision by multiplying up
	// minFee = minFee * 10^10
	minFee.Mul(minFee, precisionFactor)

	// now get fee per gas (multipled up)
	totalGasBig := big.NewInt(int64(totalGas))
	minFee.Div(minFee, totalGasBig)

	// now get as a float and divide out the precision factor
	minFeeF := big.NewFloat(0).SetInt(minFee)
	precisionFactorF := big.NewFloat(0).SetInt(precisionFactor)
	minFeeF.Quo(minFeeF, precisionFactorF)

	// now get as a native float safely
	minFeePerGas, _ := minFeeF.Float64()
	return minFeePerGas
}

func ParseMinGasError(res *sdk.TxResponse, denoms []string) (sdk.Coin, error) {
	// need to pick out the required amount
	// example: "insufficient fees; got: 0uluna required: 60000uluna: insufficient fee"
	// The approach will detect coins with valid denoms, then take the max.
	log := res.RawLog
	for _, char := range []string{";", ":", "}", "{", ")", "(", "]", "[", "."} {
		log = strings.ReplaceAll(log, char, " ")
	}
	maxFees := []sdk.Coin{}
	parts := strings.Split(log, " ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		for _, denom := range denoms {
			if denom != "" && strings.Contains(part, denom) {
				coin, err := sdk.ParseCoinNormalized(part)
				if err != nil {
					// skip
				} else if denom == coin.Denom {
					maxFees = append(maxFees, coin)
				}
			}
		}
	}

	if len(maxFees) == 0 {
		return sdk.Coin{}, errors.New("could not parse min gas error: " + res.RawLog)
	}
	// sort and take max
	sort.Slice(maxFees, func(i, j int) bool {
		return maxFees[i].Amount.GT(maxFees[j].Amount)
	})
	return maxFees[0], nil
}
