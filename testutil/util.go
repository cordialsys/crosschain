package testutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
)

func FromHex(s string) []byte {
	bz, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		panic(err)
	}
	return bz
}

func FromTimeStamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	if strings.HasSuffix(ts, "Z") {
		// use UTC timezone
		t = t.UTC()
	}
	return t
}

func HumanToBlockchain(amount string, decimals int) xc.AmountBlockchain {
	h, err := xc.NewAmountHumanReadableFromStr(amount)
	if err != nil {
		panic(err)
	}
	return h.ToBlockchain(int32(decimals))
}

func JsonPrint(a any) {
	bz, _ := json.MarshalIndent(a, "", "  ")
	fmt.Println(string(bz))
}
func Ref[T any](s T) *T {
	return &s
}
