package xlm

import (
	"strings"
	"time"

	"github.com/cordialsys/crosschain/client/errors"
	"github.com/stellar/go/xdr"
)

// TimeBounds represents the time window during which a Stellar transaction is considered valid.
//
// MinTime and MaxTime represent Stellar timebounds - a window of time over which the Transaction will be
// considered valid. In general, almost all Transactions benefit from setting an upper timebound, because once submitted,
// the status of a pending Transaction may remain unresolved for a long time if the network is congested.
// With an upper timebound, the submitter has a guaranteed time at which the Transaction is known to have either
// succeeded or failed, and can then take appropriate action (e.g. resubmit or mark as resolved).
//
// Create a TimeBounds struct using NewTimeout()
type TimeBounds struct {
	MinTime  int64
	MaxTime  int64
	wasBuilt bool
}

func NewTimeout(timeout time.Duration) TimeBounds {
	return TimeBounds{0, time.Now().Add(timeout).Unix(), true}
}

func NewInfiniteTimeout() TimeBounds {
	return TimeBounds{0, int64(0), false}
}

// Preconditions is a container for all transaction preconditions.
type Preconditions struct {
	// Transaction is only valid during a certain time range (units are seconds).
	TimeBounds TimeBounds
	// Transaction is valid for ledger numbers n such that minLedger <= n
	MinLedgerSequence int64
}

func (prec Preconditions) BuildXDR() xdr.Preconditions {
	xdrCond := xdr.Preconditions{}
	xdrTimeBounds := xdr.TimeBounds{
		MinTime: xdr.TimePoint(prec.TimeBounds.MinTime),
		MaxTime: xdr.TimePoint(prec.TimeBounds.MaxTime),
	}

	xdrCond.Type = xdr.PreconditionTypePrecondTime
	xdrCond.TimeBounds = &xdrTimeBounds

	return xdrCond
}

func CheckError(err error) errors.Status {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") {
		return errors.NetworkError
	}
	return ""
}
