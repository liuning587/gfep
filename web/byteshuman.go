package web

import (
	"fmt"
	"strconv"
)

// FormatBytesHuman 将字节数格式化为 K/M/G 等（1024 进制，与终端表一致）。
func FormatBytesHuman(n int64) string {
	if n < 0 {
		n = 0
	}
	u := uint64(n)
	if u < 1024 {
		return strconv.FormatUint(u, 10)
	}
	v := float64(u)
	units := []string{"K", "M", "G", "T", "P"}
	ui := -1
	for v >= 1024 && ui < len(units)-1 {
		v /= 1024
		ui++
	}
	suf := units[ui]
	if v >= 100 {
		return fmt.Sprintf("%.0f%s", v, suf)
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f%s", v, suf)
	}
	return fmt.Sprintf("%.2f%s", v, suf)
}
