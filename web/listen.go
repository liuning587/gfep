package web

import (
	"gfep/internal/logx"
	"gfep/internal/netaddr"
	"gfep/utils"
	"net"
	"strconv"
	"time"

	"net/http"
)

// ListenAddr 由配置解析 Web 控制台监听地址（与原先 LogWebHost/LogWebPort 语义一致）。
func ListenAddr() string {
	host := netaddr.NormalizeHostForJoin(utils.GlobalObject.LogWebHost)
	if host == "" {
		host = "0.0.0.0"
	}
	port := utils.GlobalObject.LogWebPort
	if port <= 0 {
		port = 20084
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// StartConsole 在独立 goroutine 中监听 HTTP；p 由 fep 注入终端/指标数据。
func StartConsole(addr string, absLogRoot string, p *Provider) {
	_ = TryBootstrapUsers()
	EnsureBlacklistLoaded()
	if err := ReloadTerminalBlacklist(); err != nil {
		logx.Errorf("web: blacklist: %v", err)
	}
	InitLive()
	srv := &Server{AbsLogRoot: absLogRoot, Provider: p}
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			PruneSessions()
		}
	}()
	h := srv.Routes()
	logx.Printf("web console listen http://%s (log root %s)", addr, absLogRoot)
	if err := http.ListenAndServe(addr, h); err != nil {
		logx.Errorf("web console: %v", err)
	}
}
