package utils

import (
	"fmt"
	"strconv"
)

func ParseBitrate(bitrateStr string) (int, error) {
	var bitrate int
	var multiplier int = 1

	if len(bitrateStr) == 0 {
		return 0, nil
	}

	lastChar := bitrateStr[len(bitrateStr)-1]
	switch lastChar {
	case 'K', 'k':
		multiplier = 1000
		bitrateStr = bitrateStr[:len(bitrateStr)-1]
	case 'M', 'm':
		multiplier = 1000000
		bitrateStr = bitrateStr[:len(bitrateStr)-1]
	case 'G', 'g':
		multiplier = 1000000000
		bitrateStr = bitrateStr[:len(bitrateStr)-1]
	}

	var err error
	bitrate, err = strconv.Atoi(bitrateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid bitrate format: %s", bitrateStr)
	}

	return bitrate * multiplier, nil
}
