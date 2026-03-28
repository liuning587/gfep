package main

import (
	"gfep/internal/logx"
	"gfep/internal/netaddr"
	"gfep/utils"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
)

// startLogWebIfEnabled 按配置在后台监听 HTTP，对 LogDir 做静态文件服务（列举与下载）。
func startLogWebIfEnabled() {
	if !utils.GlobalObject.LogWebEnabled {
		return
	}
	host := netaddr.NormalizeHostForJoin(utils.GlobalObject.LogWebHost)
	if host == "" {
		host = "0.0.0.0"
	}
	port := utils.GlobalObject.LogWebPort
	if port <= 0 {
		port = 20084
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
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	go func() {
		logx.Printf("log web enabled, root=%s listen http://%s", absRoot, addr)
		h := http.FileServer(http.Dir(absRoot))
		if err := http.ListenAndServe(addr, h); err != nil {
			logx.Errorf("log web: ListenAndServe: %v", err)
		}
	}()
}
