package utils

// BuildTime 二进制构建时间戳（可读字符串）。
// 发布构建可通过 -ldflags "-X gfep/utils.BuildTime=..." 注入；留空时 Web 总览会尝试使用 go build 嵌入的 vcs.time。
var BuildTime string
