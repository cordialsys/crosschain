package builder

// SubmitArgs contains options used when submitting a signed transaction.
type SubmitArgs struct {
	// Commitment selects the preflight commitment level on chains that support it.
	Commitment string
}
