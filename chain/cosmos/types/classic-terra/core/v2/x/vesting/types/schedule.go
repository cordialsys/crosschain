package types

// Imported from https://github.com/terra-money/classic-core/blob/main/x/vesting/types/schedule.go
// Some code deleted for compatibility

import (
	math "cosmossdk.io/math"
)

//-----------------------------------------------------------------------------
// Schedule

// NewSchedule returns new Schedule instance
func NewSchedule(startTime, endTime int64, ratio math.LegacyDec) Schedule {
	return Schedule{
		StartTime: startTime,
		EndTime:   endTime,
		Ratio:     ratio,
	}
}

// GetStartTime returns start time
func (s Schedule) GetStartTime() int64 {
	return s.StartTime
}

// GetEndTime returns end time
func (s Schedule) GetEndTime() int64 {
	return s.EndTime
}

// GetRatio returns ratio
func (s Schedule) GetRatio() math.LegacyDec {
	return s.Ratio
}

// Validate checks that the lazy schedule is valid.
func (s Schedule) Validate() error {
	return nil
}

// Schedules stores all lazy schedules
type Schedules []Schedule

//-----------------------------------------------------------------------------
// Vesting Schedule

// NewVestingSchedule creates a new vesting lazy schedule instance.
func NewVestingSchedule(denom string, schedules Schedules) VestingSchedule {
	return VestingSchedule{
		Denom:     denom,
		Schedules: schedules,
	}
}

// GetVestedRatio returns the ratio of tokens that have vested by blockTime.
func (vs VestingSchedule) GetVestedRatio(blockTime int64) math.LegacyDec {
	return math.LegacyDec{}
}

// GetDenom returns the denom of vesting schedule
func (vs VestingSchedule) GetDenom() string {
	return vs.Denom
}

// Validate checks that the vesting lazy schedule is valid.
func (vs VestingSchedule) Validate() error {
	return nil
}

// VestingSchedules stores all vesting schedules passed as part of a LazyGradedVestingAccount
type VestingSchedules []VestingSchedule
