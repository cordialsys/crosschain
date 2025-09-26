package tx_input

import (
	xc "github.com/cordialsys/crosschain"
)

// Per chain min
func MinFeePerByte(chain *xc.ChainConfig) float64 {
	if chain.ChainMinGasPrice >= 1 {
		return chain.ChainMinGasPrice
	}
	return 8
}

// Previously we would apply multiplier during estimation, rather than during tx-building/serialization.
// This should be removed with the legacy endpoints.
func LegacyFeeFilter(chain *xc.ChainConfig, satsPerByte float64, multiplier float64, maxGasPrice float64) float64 {
	minSatsPerByte := MinFeePerByte(chain)
	if satsPerByte < minSatsPerByte {
		satsPerByte = minSatsPerByte
	}
	defaultMultiplier := 1.0
	if multiplier < 0.01 {
		multiplier = defaultMultiplier
	}
	satsPerByteFloat := float64(satsPerByte)
	satsPerByteFloat *= multiplier

	max := maxGasPrice
	if max < 0.01 {
		// max 10k sats/byte
		max = 10000
	}
	if satsPerByteFloat > max {
		satsPerByteFloat = max
	}
	return satsPerByteFloat
}
