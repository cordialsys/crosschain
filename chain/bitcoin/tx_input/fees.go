package tx_input

// Previously we would apply multiplier during estimation, rather than during tx-building/serialization.
// This should be removed with the legacy endpoints.
func LegacyFeeFilter(satsPerByte uint64, multiplier float64, maxGasPrice float64) uint64 {
	// Min 10 sats/byte
	if satsPerByte < 10 {
		satsPerByte = 10
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
	return uint64(satsPerByteFloat)
}
