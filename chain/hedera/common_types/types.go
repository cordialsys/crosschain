package commontypes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/cordialsys/hedera-protobufs-go/common"
	"github.com/cordialsys/hedera-protobufs-go/services"
)

const MAX_MEMO_LENGTH = 100

func HederaIdToParts(s string) ([3]int64, error) {
	parts := [3]int64{0, 0, 0}
	accIdParts := strings.Split(s, ".")
	if len(accIdParts) != 3 {
		return parts, fmt.Errorf("invalid id: %s", s)
	}

	for i, a := range accIdParts {
		intAccId, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			return parts, fmt.Errorf("failed to convert account id part to int: %w", err)
		}
		parts[i] = intAccId
	}

	return parts, nil
}

func NewAccountId(acc string) (*common.AccountID, error) {
	// evm account id
	if strings.HasPrefix(acc, "0x") {
		var hex hex.Hex
		if err := hex.UnmarshalText([]byte(acc)); err != nil {
			return nil, fmt.Errorf("failed to unmarshal hex: %w", err)
		}
		return &common.AccountID{
			Account: &common.AccountID_Alias{
				Alias: hex.Bytes(),
			},
		}, nil
	} else {
		return NewHederaAccountId(acc)
	}
}

func NewHederaAccountId(acc string) (*common.AccountID, error) {
	parts, err := HederaIdToParts(acc)
	if err != nil {
		return nil, fmt.Errorf("failed to split account string: %w", err)
	}
	return &common.AccountID{
		ShardNum: parts[0],
		RealmNum: parts[1],
		Account: &common.AccountID_AccountNum{
			AccountNum: parts[2],
		},
	}, nil
}

func NewTransactionId(acc string, ts time.Time) (*common.TransactionID, error) {
	accId, err := NewAccountId(acc)
	if err != nil {
		return nil, err
	}

	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return &common.TransactionID{
		TransactionValidStart: &common.Timestamp{
			Seconds: seconds,
			Nanos:   int32(nanos),
		},
		AccountID: accId,
		Scheduled: false,
		Nonce:     0,
	}, nil
}

func NewTokenId(contract string) (*common.TokenID, error) {
	parts, err := HederaIdToParts(contract)
	if err != nil {
		return nil, fmt.Errorf("failed to split contract id: %w", err)
	}
	return &common.TokenID{
		ShardNum: parts[0],
		RealmNum: parts[1],
		TokenNum: parts[2],
	}, nil
}

type GrpcError int32

var _ error = GrpcError(0)

func (e GrpcError) Error() string {
	return services.ResponseCodeEnum_name[int32(e)]
}
