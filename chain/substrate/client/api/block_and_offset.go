package api

import (
	"fmt"
	"strconv"
	"strings"
)

type BlockAndOffset string

func (s BlockAndOffset) Parse() (uint64, int, error) {
	parts := strings.Split(string(s), "-")
	height, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("extrinsic ID contained invalid block-height: %s", parts[0])
	}
	offset, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("extrinsic ID contained invalid offset: %s", parts[1])
	}
	return height, int(offset), nil
}
