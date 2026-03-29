package utils

import "time"

// ProcessStartedAt 由 fep.Main 入口处赋值，供 Web 总览等展示进程运行时长。
var ProcessStartedAt time.Time
