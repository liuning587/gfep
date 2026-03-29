package fep

import (
	"gfep/internal/logx"
	"gfep/utils"
	"gfep/web"
	"path/filepath"
)

func fepWebProvider() *web.Provider {
	return &web.Provider{
		HostStatus:      fepWebHostStatus,
		Terminals:       fepWebTerminalRows,
		Apps:            fepWebAppRows,
		Bridges:         fepWebBridgeRows,
		TerminalCounts:  fepWebTerminalCounts,
		AppCounts:       fepWebAppCounts,
		TrafficSnapshot: fepWebTrafficSnapshot,
		KickTerminal:    fepWebKickTerminal,
	}
}

// startLogWebIfEnabled 按配置在后台监听 HTTP：管理控制台（独立 web 包）+ 安全日志下载。
func startLogWebIfEnabled() {
	if !utils.GlobalObject.LogWebEnabled {
		return
	}
	root := utils.GlobalObject.LogDir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		logx.Errorf("log web: resolve LogDir: %v", err)
		return
	}
	startTrafficHistorySampler()
	go web.StartConsole(web.ListenAddr(), absRoot, fepWebProvider())
}
