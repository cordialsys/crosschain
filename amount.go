package crosschain

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

const FLOAT_PRECISION = 6

// AmountBlockchain is a big integer amount as blockchain expects it for tx.
type AmountBlockchain big.Int

// AmountHumanReadable is a decimal amount as a human expects it for readability.
type AmountHumanReadable decimal.Decimal

func (amount AmountBlockchain) Bytes() []byte {
	bigInt := big.Int(amount)
	return bigInt.Bytes()
}

func (amount AmountBlockchain) String() string {
	bigInt := big.Int(amount)
	return bigInt.String()
}

// Int converts an AmountBlockchain into *bit.Int
func (amount AmountBlockchain) Int() *big.Int {
	bigInt := big.Int(amount)
	return &bigInt
}

func (amount AmountBlockchain) Sign() int {
	bigInt := big.Int(amount)
	return bigInt.Sign()
}

// Uint64 converts an AmountBlockchain into uint64
func (amount AmountBlockchain) Uint64() uint64 {
	bigInt := big.Int(amount)
	return bigInt.Uint64()
}

// UnmaskFloat64 converts an AmountBlockchain into float64 given the number of decimals
func (amount AmountBlockchain) UnmaskFloat64() float64 {
	bigInt := big.Int(amount)
	bigFloat := new(big.Float).SetInt(&bigInt)
	exponent := new(big.Float).SetFloat64(math.Pow10(FLOAT_PRECISION))
	bigFloat = bigFloat.Quo(bigFloat, exponent)
	f64, _ := bigFloat.Float64()
	return f64
}

// Use the underlying big.Int.Cmp()
func (amount *AmountBlockchain) Cmp(other *AmountBlockchain) int {
	return amount.Int().Cmp(other.Int())
}

// Use the underlying big.Int.Add()
func (amount *AmountBlockchain) Add(x *AmountBlockchain) AmountBlockchain {
	sum := new(big.Int)
	sum.Set((*big.Int)(amount))
	return AmountBlockchain(*sum.Add(sum, x.Int()))
}

// Use the underlying big.Int.Sub()
func (amount *AmountBlockchain) Sub(x *AmountBlockchain) AmountBlockchain {
	diff := new(big.Int)
	diff.Set((*big.Int)(amount))
	return AmountBlockchain(*diff.Sub(diff, x.Int()))
}

// Use the underlying big.Int.Mul()
func (amount *AmountBlockchain) Mul(x *AmountBlockchain) AmountBlockchain {
	prod := new(big.Int)
	prod.Set((*big.Int)(amount))
	return AmountBlockchain(*prod.Mul(prod, x.Int()))
}

// Use the underlying big.Int.Div()
func (amount *AmountBlockchain) Div(x *AmountBlockchain) AmountBlockchain {
	quot := new(big.Int)
	quot.Set((*big.Int)(amount))
	return AmountBlockchain(*quot.Div(quot, x.Int()))
}

func (amount *AmountBlockchain) Abs() AmountBlockchain {
	abs := new(big.Int)
	abs.Set((*big.Int)(amount))
	return AmountBlockchain(*abs.Abs(abs))
}

var zero = big.NewInt(0)

func (amount *AmountBlockchain) IsZero() bool {
	return amount.Int().Cmp(zero) == 0
}

func (amount *AmountBlockchain) ToHuman(decimals int32) AmountHumanReadable {
	dec := decimal.NewFromBigInt(amount.Int(), -decimals)
	return AmountHumanReadable(dec)
}

func (amount AmountBlockchain) ApplyGasPriceMultiplier(chain *ChainClientConfig) AmountBlockchain {
	if chain.ChainGasMultiplier > 0.01 {
		return MultiplyByFloat(amount, chain.ChainGasMultiplier)
	}
	// no multiplier configured, return same
	return amount
}

func MultiplyByFloat(amount AmountBlockchain, multiplier float64) AmountBlockchain {
	if amount.Uint64() == 0 {
		return amount
	}
	// We are computing (100000 * multiplier * amount) / 100000
	precision := uint64(1000000)
	multBig := NewAmountBlockchainFromUint64(uint64(float64(precision) * multiplier))
	divBig := NewAmountBlockchainFromUint64(precision)
	product := multBig.Mul(&amount)
	result := product.Div(&divBig)
	return result
}

// NewAmountBlockchainFromUint64 creates a new AmountBlockchain from a uint64
func NewAmountBlockchainFromInt64(i64 int64) (AmountBlockchain, bool) {
	if i64 < 0 {
		return NewAmountBlockchainFromUint64(0), false
	}
	return NewAmountBlockchainFromUint64(uint64(i64)), true
}

// NewAmountBlockchainFromUint64 creates a new AmountBlockchain from a uint64
func NewAmountBlockchainFromUint64(u64 uint64) AmountBlockchain {
	bigInt := new(big.Int).SetUint64(u64)
	return AmountBlockchain(*bigInt)
}

// NewAmountBlockchainToMaskFloat64 creates a new AmountBlockchain as a float64 times 10^FLOAT_PRECISION
func NewAmountBlockchainToMaskFloat64(f64 float64) AmountBlockchain {
	bigFloat := new(big.Float).SetFloat64(f64)
	exponent := new(big.Float).SetFloat64(math.Pow10(FLOAT_PRECISION))
	bigFloat = bigFloat.Mul(bigFloat, exponent)
	var bigInt big.Int
	bigFloat.Int(&bigInt)
	return AmountBlockchain(bigInt)
}

// NewAmountBlockchainFromStr creates a new AmountBlockchain from a string
func NewAmountBlockchainFromStr(str string) AmountBlockchain {
	var ok bool
	var bigInt *big.Int
	bigInt, ok = new(big.Int).SetString(str, 0)
	if !ok {
		return NewAmountBlockchainFromUint64(0)
	}
	return AmountBlockchain(*bigInt)
}

// NewAmountHumanReadableFromStr creates a new AmountHumanReadable from a string
func NewAmountHumanReadableFromStr(str string) (AmountHumanReadable, error) {
	decimal, err := decimal.NewFromString(str)
	return AmountHumanReadable(decimal), err
}

// NewAmountHumanReadableFromFloat creates a new AmountHumanReadable from a float
func NewAmountHumanReadableFromFloat(float float64) AmountHumanReadable {
	return AmountHumanReadable(decimal.NewFromFloat(float))
}

func (amount AmountHumanReadable) Decimal() decimal.Decimal {
	return decimal.Decimal(amount)
}

func (amount AmountHumanReadable) ToBlockchain(decimals int32) AmountBlockchain {
	factor := decimal.NewFromInt32(10).Pow(decimal.NewFromInt32(decimals))
	raised := ((decimal.Decimal)(amount)).Mul(factor)
	return AmountBlockchain(*raised.BigInt())
}

func (amount AmountHumanReadable) String() string {
	return decimal.Decimal(amount).String()
}

func (amount AmountHumanReadable) Div(x AmountHumanReadable) AmountHumanReadable {
	return AmountHumanReadable(decimal.Decimal(amount).Div(decimal.Decimal(x)))
}

var _ json.Marshaler = AmountHumanReadable{}
var _ json.Unmarshaler = &AmountHumanReadable{}
var _ yaml.Unmarshaler = &AmountHumanReadable{}
var _ yaml.Marshaler = AmountHumanReadable{}
var _ yaml.IsZeroer = AmountHumanReadable{}

func (b AmountHumanReadable) MarshalYAML() (interface{}, error) {
	return b.String(), nil
}

func (b AmountHumanReadable) IsZero() bool {
	return decimal.Decimal(b).IsZero()
}

func (b *AmountHumanReadable) UnmarshalYAML(node *yaml.Node) error {
	value := node.Value
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "\"")
	value = strings.TrimSuffix(value, "\"")
	dec, err := decimal.NewFromString(value)
	if err != nil {
		return fmt.Errorf("invalid decimal amount: %v", err)
	}
	*b = AmountHumanReadable(dec)
	return nil
}

func (b AmountHumanReadable) MarshalJSON() ([]byte, error) {
	return []byte("\"" + b.String() + "\""), nil
}

func (b *AmountHumanReadable) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	str := strings.Trim(string(p), "\"")
	decimal, err := decimal.NewFromString(str)
	if err != nil {
		return err
	}
	*b = AmountHumanReadable(decimal)
	return nil
}

var _ json.Marshaler = AmountBlockchain{}
var _ json.Unmarshaler = &AmountBlockchain{}

func (b AmountBlockchain) MarshalJSON() ([]byte, error) {
	return []byte("\"" + b.String() + "\""), nil
}

func (b *AmountBlockchain) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	str := strings.Trim(string(p), "\"")
	var z big.Int
	_, ok := z.SetString(str, 0)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", p)
	}
	*b = AmountBlockchain(z)
	return nil
}
