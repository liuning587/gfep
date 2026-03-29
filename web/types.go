package web

// GoRuntimeStatus Go 进程内 runtime 采样（与主机 OS 指标区分）。
type GoRuntimeStatus struct {
	Goroutines    int     `json:"goroutines"`              // runtime.NumGoroutine
	HeapAllocMiB  float64 `json:"heapAllocMiB"`            // 当前堆对象占用（近似进程堆上数据量）
	HeapInuseMiB  float64 `json:"heapInuseMiB"`            // 堆中正在使用的 span
	HeapSysMiB    float64 `json:"heapSysMiB"`              // 从 OS 为堆保留的内存
	SysMiB        float64 `json:"sysMiB"`                  // 运行时向 OS 申请的总内存（含堆栈等）
	StackInuseMiB float64 `json:"stackInuseMiB,omitempty"` // 协程栈使用中估算
	NumGC         uint32  `json:"numGC"`                   // 累计 GC 次数
	LastGCPauseMs float64 `json:"lastGCPauseMs"`           // 上次 STW 暂停（毫秒）
	// GCCPUFraction 自进程启动以来 GC 占用 CPU 时间的比例（0~1，宜长期观察，非瞬时）
	GCCPUFraction float64 `json:"gcCPUFraction"`
	// NextGCMiB 下次 GC 的目标堆存活量（MemStats.NextGC，字节转 MiB）
	NextGCMiB float64 `json:"nextGCMiB"`
	// MSpan / MCache：运行时元数据内存（MemStats.MSpan* / MCache*）
	MSpanInuseMiB  float64 `json:"mspanInuseMiB"`
	MSpanSysMiB    float64 `json:"mspanSysMiB"`
	MCacheInuseMiB float64 `json:"mcacheInuseMiB"`
	MCacheSysMiB   float64 `json:"mcacheSysMiB"`
}

// HostStatus 控制台总览中的主机采样（JSON 字段名保持稳定）。
type HostStatus struct {
	CPUPercent      float64 `json:"cpuPercent"`
	MemUsedPercent  float64 `json:"memUsedPercent"`
	MemUsedBytes    int64   `json:"memUsedBytes,omitempty"`
	MemTotalBytes   int64   `json:"memTotalBytes,omitempty"`
	DiskUsedPercent float64 `json:"diskUsedPercent"`
	DiskUsedBytes   int64   `json:"diskUsedBytes,omitempty"`
	DiskTotalBytes  int64   `json:"diskTotalBytes,omitempty"`
	DiskPath        string  `json:"diskPath"`
	// HostUptimeSec 操作系统自启动以来的秒数（gopsutil/host）。
	HostUptimeSec uint64 `json:"hostUptimeSec,omitempty"`
	// HostBootTimeUtc 主机开机时刻（控制台按 Asia/Shanghai 格式化为可读串；JSON 字段名历史保留）。
	HostBootTimeUtc string          `json:"hostBootTimeUtc,omitempty"`
	GoRuntime       GoRuntimeStatus `json:"goRuntime,omitempty"`
}

// TrafficStatus 总览用：当前在线连接的累计流量 + 最近 15 个完整分钟的每分钟总字节。
type TrafficStatus struct {
	TerminalTotalRx     string   `json:"terminalTotalRx"`
	TerminalTotalTx     string   `json:"terminalTotalTx"`
	AppTotalRx          string   `json:"appTotalRx"`
	AppTotalTx          string   `json:"appTotalTx"`
	TerminalBytesPerMin []uint64 `json:"terminalBytesPerMin"`
	AppBytesPerMin      []uint64 `json:"appBytesPerMin"`
}

// TerminalRow 在线终端表一行。
type TerminalRow struct {
	ConnID           uint32  `json:"connId"`
	RemoteTCP        string  `json:"remoteTcp"`
	Protocol         string  `json:"protocol"`
	Addr             string  `json:"addr"`
	ConnTime         *string `json:"connTime,omitempty"`
	OnlineDuration   string  `json:"onlineDuration,omitempty"` // 相对连接建立时刻的时长文案
	LoginTime        *string `json:"loginTime,omitempty"`
	HeartbeatTime    *string `json:"heartbeatTime,omitempty"`
	LastRxTime       *string `json:"lastRxTime,omitempty"`
	LastTxTime       *string `json:"lastTxTime,omitempty"`
	LastReportTime   *string `json:"lastReportTime,omitempty"`
	UplinkMsgCount   string  `json:"uplinkMsgCount"`
	DownlinkMsgCount string  `json:"downlinkMsgCount"`
	UplinkBytes      string  `json:"uplinkBytes"`
	DownlinkBytes    string  `json:"downlinkBytes"`
}

// AppRow 主站/APP 连接表一行。
type AppRow struct {
	ConnID           uint32  `json:"connId"`
	RemoteTCP        string  `json:"remoteTcp"`
	Protocol         string  `json:"protocol"`
	MasterSummary    string  `json:"masterSummary"`
	ConnTime         *string `json:"connTime,omitempty"`
	OnlineDuration   string  `json:"onlineDuration,omitempty"` // 自 TCP 建立起的在线时长（中文）
	LastRxTime       *string `json:"lastRxTime,omitempty"`
	LastTxTime       *string `json:"lastTxTime,omitempty"`
	LastReportTime   *string `json:"lastReportTime,omitempty"`
	UplinkMsgCount   string  `json:"uplinkMsgCount"`
	DownlinkMsgCount string  `json:"downlinkMsgCount"`
	UplinkBytes      string  `json:"uplinkBytes"`
	DownlinkBytes    string  `json:"downlinkBytes"`
}

// Provider 由 fep 注入：终端/主站快照与主机指标，避免 web 包依赖 fep 产生循环引用。
type Provider struct {
	HostStatus     func() HostStatus
	Terminals      func(expand bool, protoFilter, query string) []TerminalRow
	Apps           func(query string) []AppRow
	TerminalCounts func() map[string]int
	AppCounts      func() map[string]int
	// TrafficSnapshot 当前连接累计收/发字节与近 15 分钟每分钟流量（Web 未启用采样时可为 nil）。
	TrafficSnapshot func() TrafficStatus
	// KickTerminal 关闭指定 connId 的 TCP（仅允许当前登记在终端 registry 中的连接）。
	KickTerminal func(connID uint32) error
}
