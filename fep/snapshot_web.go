package fep

import (
	"errors"
	"fmt"
	"gfep/bridge"
	"gfep/utils"
	"gfep/web"
	"gfep/ziface"
	"gfep/znet"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

func u64dec(n uint64) string {
	return strconv.FormatUint(n, 10)
}

// formatBytesHuman 将字节数格式化为 K/M/G 等（1024 进制）。
func formatBytesHuman(n uint64) string {
	if n < 1024 {
		return strconv.FormatUint(n, 10)
	}
	v := float64(n)
	units := []string{"K", "M", "G", "T", "P"}
	u := -1
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	suf := units[u]
	if v >= 100 {
		return fmt.Sprintf("%.0f%s", v, suf)
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f%s", v, suf)
	}
	return fmt.Sprintf("%.2f%s", v, suf)
}

func isTerminalRegistryConn(connID uint32) bool {
	if _, ok := regTmn376.addrForConn(connID); ok {
		return true
	}
	if _, ok := regTmn698.addrForConn(connID); ok {
		return true
	}
	_, ok := regTmnNw.addrForConn(connID)
	return ok
}

// fepWebKickTerminal 主动断开终端 TCP（connId 须当前登记在任一端规约终端 registry 中）。
func fepWebKickTerminal(connID uint32) error {
	if connID == 0 {
		return errors.New("无效 connId")
	}
	if !isTerminalRegistryConn(connID) {
		return errors.New("非终端连接或已不在线")
	}
	srv := utils.GlobalObject.TCPServer
	if srv == nil {
		return errors.New("TCP 服务未就绪")
	}
	ic, err := srv.GetConnMgr().Get(connID)
	if err != nil {
		return errors.New("连接不存在或已关闭")
	}
	go ic.Stop()
	return nil
}

// formatOnlineSince 将连接建立时间转为「在线时长」中文文案（相对当前时刻）。
func formatOnlineSince(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	sec := int64(d.Round(time.Second) / time.Second)
	if sec < 60 {
		return strconv.FormatInt(sec, 10) + "秒"
	}
	if sec < 3600 {
		m := sec / 60
		s := sec % 60
		if s == 0 {
			return strconv.FormatInt(m, 10) + "分钟"
		}
		return fmt.Sprintf("%d分%d秒", m, s)
	}
	if sec < 86400 {
		h := sec / 3600
		m := (sec % 3600) / 60
		s := sec % 60
		return fmt.Sprintf("%d小时%d分%d秒", h, m, s)
	}
	days := sec / 86400
	rem := sec % 86400
	h := rem / 3600
	m := (rem % 3600) / 60
	return fmt.Sprintf("%d天%d小时%d分", days, h, m)
}

const (
	hostStatusTTL       = 2 * time.Second
	hostStatusCPUSample = 200 * time.Millisecond
)

var (
	hostStatusMu     sync.Mutex
	hostStatusCached web.HostStatus
	hostStatusAt     time.Time
)

// diskPathForOverview 总览 JSON 中的路径展示：配置为相对路径则原样规范化；绝对路径则尽量相对当前工作目录，否则退回末级目录名。
func diskPathForOverview(configured, absForUsage string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "."
	}
	if !filepath.IsAbs(configured) {
		return filepath.ToSlash(filepath.Clean(configured))
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(filepath.Base(absForUsage))
	}
	rel, err := filepath.Rel(wd, absForUsage)
	if err != nil {
		return filepath.ToSlash(filepath.Base(absForUsage))
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return "."
	}
	if strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Base(absForUsage))
	}
	return filepath.ToSlash(rel)
}

func fepWebHostStatus() web.HostStatus {
	hostStatusMu.Lock()
	defer hostStatusMu.Unlock()
	if time.Since(hostStatusAt) < hostStatusTTL {
		return hostStatusCached
	}
	cfgLog := strings.TrimSpace(utils.GlobalObject.LogDir)
	diskUsagePath := "."
	if cfgLog != "" {
		diskUsagePath = cfgLog
	}
	if abs, err := filepath.Abs(diskUsagePath); err == nil {
		diskUsagePath = abs
	}
	out := web.HostStatus{DiskPath: diskPathForOverview(utils.GlobalObject.LogDir, diskUsagePath)}
	if pct, err := cpu.Percent(hostStatusCPUSample, false); err == nil && len(pct) > 0 {
		out.CPUPercent = pct[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		out.MemUsedPercent = vm.UsedPercent
		out.MemUsedBytes = int64(vm.Used)
		out.MemTotalBytes = int64(vm.Total)
	}
	if du, err := disk.Usage(diskUsagePath); err == nil {
		out.DiskUsedPercent = du.UsedPercent
		out.DiskUsedBytes = int64(du.Used)
		out.DiskTotalBytes = int64(du.Total)
	}
	if ut, err := host.Uptime(); err == nil {
		out.HostUptimeSec = ut
	}
	if bt, err := host.BootTime(); err == nil && bt > 0 {
		out.HostBootTimeUtc = web.FormatDisplayWeb(time.Unix(int64(bt), 0))
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	const mib = 1024 * 1024
	lastGCPauseMs := 0.0
	if ms.NumGC > 0 {
		lastGCPauseMs = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e6
	}
	out.GoRuntime = web.GoRuntimeStatus{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocMiB:   float64(ms.HeapAlloc) / mib,
		HeapInuseMiB:   float64(ms.HeapInuse) / mib,
		HeapSysMiB:     float64(ms.HeapSys) / mib,
		SysMiB:         float64(ms.Sys) / mib,
		StackInuseMiB:  float64(ms.StackInuse) / mib,
		NumGC:          ms.NumGC,
		LastGCPauseMs:  lastGCPauseMs,
		GCCPUFraction:  ms.GCCPUFraction,
		NextGCMiB:      float64(ms.NextGC) / mib,
		MSpanInuseMiB:  float64(ms.MSpanInuse) / mib,
		MSpanSysMiB:    float64(ms.MSpanSys) / mib,
		MCacheInuseMiB: float64(ms.MCacheInuse) / mib,
		MCacheSysMiB:   float64(ms.MCacheSys) / mib,
	}
	hostStatusCached = out
	hostStatusAt = time.Now()
	return out
}

func connDetailsOrEmpty(id uint32) (znet.ConnDetails, bool) {
	srv := utils.GlobalObject.TCPServer
	if srv == nil {
		return znet.ConnDetails{}, false
	}
	ic, err := srv.GetConnMgr().Get(id)
	if err != nil {
		return znet.ConnDetails{}, false
	}
	co, ok := ic.(*znet.Connection)
	if !ok {
		return znet.ConnDetails{}, false
	}
	return co.Details(), true
}

func terminalRowFromDetails(protocol string, d znet.ConnDetails) web.TerminalRow {
	return web.TerminalRow{
		ConnID:           d.ConnID,
		RemoteTCP:        d.RemoteTCP,
		Protocol:         protocol,
		Addr:             d.TermAddr,
		ConnTime:         web.FormatDisplayWebPtr(d.Ctime),
		OnlineDuration:   formatOnlineSince(d.Ctime),
		LoginTime:        web.FormatDisplayWebPtr(d.Ltime),
		HeartbeatTime:    web.FormatDisplayWebPtr(d.Htime),
		LastRxTime:       web.FormatDisplayWebPtr(d.Rtime),
		LastTxTime:       web.FormatDisplayWebPtr(d.LastTxAt),
		LastReportTime:   web.FormatDisplayWebPtr(d.LastReportAt),
		UplinkMsgCount:   u64dec(d.RxFrames),
		DownlinkMsgCount: u64dec(d.TxWrites),
		UplinkBytes:      formatBytesHuman(d.RxFrameBytes),
		DownlinkBytes:    formatBytesHuman(d.TxWriteBytes),
	}
}

func appRowFromDetails(protocol string, d znet.ConnDetails, msa string) web.AppRow {
	local := utils.GlobalObject.Host + ":" + strconv.Itoa(utils.GlobalObject.TCPPort)
	summary := msa
	if summary == "" {
		summary = d.RemoteTCP
	} else if d.RemoteTCP != "" {
		summary = msa + " · " + d.RemoteTCP
	}
	return web.AppRow{
		ConnID:           d.ConnID,
		RemoteTCP:        d.RemoteTCP,
		Protocol:         protocol,
		MasterSummary:    summary + " · 监听 " + local,
		ConnTime:         web.FormatDisplayWebPtr(d.Ctime),
		OnlineDuration:   formatOnlineSince(d.Ctime),
		LastRxTime:       web.FormatDisplayWebPtr(d.Rtime),
		LastTxTime:       web.FormatDisplayWebPtr(d.LastTxAt),
		LastReportTime:   web.FormatDisplayWebPtr(d.LastReportAt),
		UplinkMsgCount:   u64dec(d.RxFrames),
		DownlinkMsgCount: u64dec(d.TxWrites),
		UplinkBytes:      u64dec(d.RxFrameBytes),
		DownlinkBytes:    u64dec(d.TxWriteBytes),
	}
}

func fepWebTerminalRows(expand bool, protoFilter, query string) []web.TerminalRow {
	type item struct {
		proto string
		ac    addrConnID
	}
	var items []item
	for _, t := range regTmn376.snapshot() {
		items = append(items, item{"376.1", t})
	}
	for _, t := range regTmn698.snapshot() {
		items = append(items, item{"698.45", t})
	}
	for _, t := range regTmnNw.snapshot() {
		items = append(items, item{"NW", t})
	}

	buildRows := func() []web.TerminalRow {
		var rows []web.TerminalRow
		for _, it := range items {
			if protoFilter != "" && !strings.EqualFold(strings.TrimSpace(protoFilter), it.proto) {
				continue
			}
			d, ok := connDetailsOrEmpty(it.ac.connID)
			if !ok {
				continue
			}
			row := terminalRowFromDetails(it.proto, d)
			q := strings.TrimSpace(query)
			if q != "" {
				if !strings.Contains(strings.ToLower(row.Addr), strings.ToLower(q)) &&
					!strings.Contains(strings.ToLower(row.RemoteTCP), strings.ToLower(q)) {
					continue
				}
			}
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool {
			li, lj := rows[i].LoginTime, rows[j].LoginTime
			if li == nil && lj == nil {
				return rows[i].ConnID < rows[j].ConnID
			}
			if li == nil {
				return true
			}
			if lj == nil {
				return false
			}
			return *li > *lj
		})
		return rows
	}

	if expand {
		return buildRows()
	}

	best := make(map[string]web.TerminalRow)
	for _, row := range buildRows() {
		key := row.Protocol + "\x00" + row.Addr
		cur, ok := best[key]
		if !ok {
			best[key] = row
			continue
		}
		if row.LoginTime == nil && cur.LoginTime == nil {
			if row.ConnID > cur.ConnID {
				best[key] = row
			}
			continue
		}
		if row.LoginTime == nil {
			continue
		}
		if cur.LoginTime == nil {
			best[key] = row
			continue
		}
		if *row.LoginTime > *cur.LoginTime || (*row.LoginTime == *cur.LoginTime && row.ConnID > cur.ConnID) {
			best[key] = row
		}
	}
	out := make([]web.TerminalRow, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := out[i].LoginTime, out[j].LoginTime
		if li == nil && lj == nil {
			return out[i].ConnID < out[j].ConnID
		}
		if li == nil {
			return true
		}
		if lj == nil {
			return false
		}
		return *li > *lj
	})
	return out
}

func bridgeRowFromSnap(d znet.ConnDetails, s bridge.Snapshot) web.BridgeRow {
	online := ""
	if !s.TcpSince.IsZero() {
		online = formatOnlineSince(s.TcpSince)
	}
	return web.BridgeRow{
		TerminalConnID:    d.ConnID,
		TerminalAddr:      d.TermAddr,
		TerminalRemoteTCP: d.RemoteTCP,
		BridgeHost:        s.Host,
		AddrHex:           s.AddrHex,
		Protocol:          s.ProtoLabel,
		Status:            s.Status,
		StatusText:        s.StatusZh,
		TcpSince:          web.FormatDisplayWebPtr(s.TcpSince),
		LoginTime:         web.FormatDisplayWebPtr(s.LoginTime),
		HeartbeatTime:     web.FormatDisplayWebPtr(s.HeartTime),
		LastRxTime:        web.FormatDisplayWebPtr(s.LastRx),
		LastTxTime:        web.FormatDisplayWebPtr(s.LastTx),
		OnlineDuration:    online,
		RxPkts:            u64dec(s.RxPkts),
		TxPkts:            u64dec(s.TxPkts),
		RxBytes:           web.FormatBytesHuman(int64(s.RxBytes)),
		TxBytes:           web.FormatBytesHuman(int64(s.TxBytes)),
		HeartUnAck:        s.HeartUnAck,
	}
}

// fepWebBridgeRows 遍历连接管理器，收集带 bridge.Conn 的终端连接。
func fepWebBridgeRows(query string) []web.BridgeRow {
	srv := utils.GlobalObject.TCPServer
	if srv == nil {
		return nil
	}
	q := strings.TrimSpace(query)
	var out []web.BridgeRow
	srv.GetConnMgr().Range(func(ic ziface.IConnection) {
		co, ok := ic.(*znet.Connection)
		if !ok {
			return
		}
		b, err := co.GetProperty("bridge")
		if err != nil || b == nil {
			return
		}
		bc, ok := b.(*bridge.Conn)
		if !ok {
			return
		}
		row := bridgeRowFromSnap(co.Details(), bc.Snapshot())
		if q != "" {
			f := strings.ToLower(q)
			if !strings.Contains(strings.ToLower(row.BridgeHost), f) &&
				!strings.Contains(strings.ToLower(row.TerminalAddr), f) &&
				!strings.Contains(strings.ToLower(row.TerminalRemoteTCP), f) &&
				!strings.Contains(strings.ToLower(row.AddrHex), f) &&
				!strings.Contains(strings.ToLower(row.Protocol), f) {
				return
			}
		}
		out = append(out, row)
	})
	sort.Slice(out, func(i, j int) bool { return out[i].TerminalConnID < out[j].TerminalConnID })
	return out
}

func fepWebAppRows(query string) []web.AppRow {
	type item struct {
		proto string
		ac    addrConnID
	}
	var items []item
	for _, a := range regApp376.snapshot() {
		items = append(items, item{"376-主站", a})
	}
	for _, a := range regApp698.snapshot() {
		items = append(items, item{"698-主站", a})
	}
	for _, a := range regAppNw.snapshot() {
		items = append(items, item{"Nw-主站", a})
	}

	var rows []web.AppRow
	for _, it := range items {
		d, ok := connDetailsOrEmpty(it.ac.connID)
		if !ok {
			continue
		}
		row := appRowFromDetails(it.proto, d, it.ac.addrStr)
		q := strings.TrimSpace(query)
		if q != "" {
			if !strings.Contains(strings.ToLower(row.MasterSummary), strings.ToLower(q)) &&
				!strings.Contains(strings.ToLower(row.RemoteTCP), strings.ToLower(q)) {
				continue
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ConnID < rows[j].ConnID })
	return rows
}

func fepWebAppCounts() map[string]int {
	type item struct {
		proto string
		ac    addrConnID
	}
	var items []item
	for _, a := range regApp376.snapshot() {
		items = append(items, item{"376-主站", a})
	}
	for _, a := range regApp698.snapshot() {
		items = append(items, item{"698-主站", a})
	}
	for _, a := range regAppNw.snapshot() {
		items = append(items, item{"Nw-主站", a})
	}
	m := make(map[string]int)
	for _, it := range items {
		if _, ok := connDetailsOrEmpty(it.ac.connID); ok {
			m[it.proto]++
		}
	}
	return m
}

func fepWebTerminalCounts() map[string]int {
	rows := fepWebTerminalRows(false, "", "")
	m := make(map[string]int)
	for _, r := range rows {
		m[r.Protocol]++
	}
	return m
}
