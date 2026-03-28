// Package netaddr 提供 TCP 监听与日志用的地址格式化，保证 IPv6 显示正确。
package netaddr

import (
	"net"
	"strconv"
	"strings"
)

// NormalizeHostForJoin 去掉 IPv6 外层方括号（若用户写成 [::1]），供 net.JoinHostPort 使用。
func NormalizeHostForJoin(host string) string {
	host = strings.TrimSpace(host)
	if len(host) < 3 || host[0] != '[' {
		return host
	}
	// 仅处理形如 [::] / [2001:db8::1] / [fe80::1%eth0] 的整段括号
	close := strings.LastIndexByte(host, ']')
	if close != len(host)-1 || close <= 1 {
		return host
	}
	return host[1:close]
}

// JoinListen 生成监听地址 host:port；IPv6 的 host 会正确加方括号。
func JoinListen(host string, port int) string {
	h := NormalizeHostForJoin(strings.TrimSpace(host))
	if h == "" {
		h = "0.0.0.0"
	}
	return net.JoinHostPort(h, strconv.Itoa(port))
}

// FormatTCP 将 TCP 地址格式化为 host:port，IPv6 带方括号，与 net.JoinHostPort 一致。
func FormatTCP(a net.Addr) string {
	if a == nil {
		return ""
	}
	switch v := a.(type) {
	case *net.TCPAddr:
		if v == nil {
			return ""
		}
		if v.IP == nil {
			return net.JoinHostPort("", strconv.Itoa(v.Port))
		}
		return net.JoinHostPort(v.IP.String(), strconv.Itoa(v.Port))
	default:
		return a.String()
	}
}
