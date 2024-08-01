package types

// StakeAccount represents the entire structure of the JSON response
type StakeAccount struct {
	Parsed  Parsed `json:"parsed"`
	Program string `json:"program"`
	Space   int    `json:"space"`
}

// Parsed represents the parsed data section
type Parsed struct {
	Info Info   `json:"info"`
	Type string `json:"type"`
}

// Info represents the info section
type Info struct {
	Meta  Meta  `json:"meta"`
	Stake Stake `json:"stake"`
}

// Meta represents the meta section
type Meta struct {
	Authorized        Authorized `json:"authorized"`
	Lockup            Lockup     `json:"lockup"`
	RentExemptReserve string     `json:"rentExemptReserve"`
}

// Authorized represents the authorized section
type Authorized struct {
	Staker     string `json:"staker"`
	Withdrawer string `json:"withdrawer"`
}

// Lockup represents the lockup section
type Lockup struct {
	Custodian     string `json:"custodian"`
	Epoch         int    `json:"epoch"`
	UnixTimestamp int    `json:"unixTimestamp"`
}

// Stake represents the stake section
type Stake struct {
	CreditsObserved int        `json:"creditsObserved"`
	Delegation      Delegation `json:"delegation"`
}

// Delegation represents the delegation section
type Delegation struct {
	ActivationEpoch    string  `json:"activationEpoch"`
	DeactivationEpoch  string  `json:"deactivationEpoch"`
	Stake              string  `json:"stake"`
	Voter              string  `json:"voter"`
	WarmupCooldownRate float64 `json:"warmupCooldownRate"`
}
