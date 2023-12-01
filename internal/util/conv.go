package util

import (
	"strconv"
	"strings"
)

func ByteToIntSlice(data, sep string) ([]int, error) {
	split := strings.Split(data, sep)
	res := make([]int, 0)
	for _, v := range split {
		if v == "" {
			continue
		}
		pid, err := strconv.Atoi(v)
		if err != nil {
			return res, err
		}
		res = append(res, pid)
	}
	return res, nil
}
