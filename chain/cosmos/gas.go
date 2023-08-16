package cosmos

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	xc "github.com/jumpcrypto/crosschain"
)

// EstimateGas estimates gas price for a Cosmos chain
func (client *Client) EstimateGasPrice(ctx context.Context) (float64, error) {
	zero := float64(0)

	gasLimitForEstimate := uint64(1_000_000)
	tx, err := client.BuildReferenceTransfer(gasLimitForEstimate)
	if err != nil {
		return zero, fmt.Errorf("could not build estimate gas tx: %v", err)
	}
	txBytes, _ := tx.Serialize()

	res, err := client.Ctx.BroadcastTx(txBytes)
	if err != nil {
		return zero, err
	}
	native := client.Asset.GetNativeAsset()
	denoms := []string{
		native.ChainCoin,
		native.GasCoin,
	}
	minFeeRaw, err := ParseMinGasError(res, denoms)
	if err != nil {
		defaultGas := client.Asset.GetNativeAsset().ChainGasPriceDefault
		return defaultGas, nil
	}
	// Need to convert total fee into gas price (cost per gas)
	return TotalFeeToFeePerGas(minFeeRaw.Amount.String(), gasLimitForEstimate), nil
}

// There is no way to estimate gas on cosmos chains.
// Every RPC node and validator has the ability to see their own min price.
// The only way currently to determine this price is to try to submit a tx for free
// and look at the error log.
func (client *Client) BuildReferenceTransfer(gasLimit uint64) (*Tx, error) {
	native := client.Asset.GetNativeAsset()
	builder, err := NewTxBuilder(native)
	if err != nil {
		return nil, err
	}

	kb := keyring.NewInMemory(client.Ctx.Codec)
	hdPath := hd.CreateHDPath(118, 0, 0).String()
	fromRec, _, err := kb.NewMnemonic("from", keyring.English, hdPath, "", hd.Secp256k1)
	if err != nil {
		return nil, err
	}
	toRec, _, err := kb.NewMnemonic("to", keyring.English, hdPath, "", hd.Secp256k1)
	if err != nil {
		return nil, err
	}
	fromPk, err := fromRec.GetPubKey()
	if err != nil {
		return nil, err
	}
	toPk, err := toRec.GetPubKey()
	if err != nil {
		return nil, err
	}
	from, err := sdk.Bech32ifyAddressBytes(native.ChainPrefix, getPublicKey(native, fromPk.Bytes()).Address())
	if err != nil {
		return nil, err
	}
	to, err := sdk.Bech32ifyAddressBytes(native.ChainPrefix, getPublicKey(native, toPk.Bytes()).Address())
	if err != nil {
		return nil, err
	}
	input := NewTxInput()
	input.GasLimit = gasLimit
	input.GasPrice = 0
	tx, err := builder.NewTransfer(xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(1), input)
	if err != nil {
		return nil, err
	}
	toSign, err := tx.Sighashes()
	if err != nil {
		return nil, err
	}
	sig, _, err := kb.Sign("from", toSign[0])
	if err != nil {
		return nil, err
	}
	err = tx.AddSignatures(sig)
	if err != nil {
		return nil, err
	}
	return tx.(*Tx), nil
}

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
