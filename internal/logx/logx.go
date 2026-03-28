// Package logx 提供带统一前缀的标准库日志封装，避免业务路径直接使用 fmt.Print*。
package logx

import (
	"log"
	"os"
)

// StdLogFlags 日期、时间、小数秒（标准库为微秒 6 位，形如 15:04:05.000000）。
const StdLogFlags = log.LstdFlags | log.Lmicroseconds

var std = log.New(os.Stderr, "gfep: ", StdLogFlags)

// Printf 输出一行（与标准库 log 相同前缀）。
func Printf(format string, v ...any) {
	std.Printf(format, v...)
}

// Println 输出一行。
func Println(v ...any) {
	std.Println(v...)
}

// Infof 信息级别（当前与 Printf 相同输出目标）。
func Infof(format string, v ...any) {
	std.Printf(format, v...)
}

// Warnf 告警。
func Warnf(format string, v ...any) {
	std.Printf("[WARN] "+format, v...)
}

// Errorf 错误。
func Errorf(format string, v ...any) {
	std.Printf("[ERROR] "+format, v...)
}
