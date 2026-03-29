package web

import (
	"strings"

	"gfep/utils"
)

// buildEffectiveConfigMap 与控制台展示一致的有效配置快照（原始值）。
func buildEffectiveConfigMap() map[string]any {
	g := utils.GlobalObject
	return map[string]any{
		"Name": g.Name, "Host": g.Host, "TCPPort": g.TCPPort, "TCPNetwork": g.TCPNetwork,
		"BridgeHost698": g.BridgeHost698, "MaxConn": g.MaxConn,
		"WorkerPoolSize": g.WorkerPoolSize, "MaxWorkerTaskLen": g.MaxWorkerTaskLen, "MaxMsgChanLen": g.MaxMsgChanLen,
		"LogDir": g.LogDir, "LogFile": g.LogFile, "LogWebEnabled": g.LogWebEnabled, "LogWebHost": g.LogWebHost, "LogWebPort": g.LogWebPort,
		"LogWebSessionIdleMin": g.LogWebSessionIdleMin,
		"LogDebugClose":        g.LogDebugClose, "LogConnTrace": g.LogConnTrace, "LogNetVerbose": g.LogNetVerbose,
		"LogPacketHex": g.LogPacketHex, "LogLinkLayer": g.LogLinkLayer, "LogForwardEgressHex": g.LogForwardEgressHex,
		"ForwardWorkers": g.ForwardWorkers, "ForwardQueueLen": g.ForwardQueueLen,
		"FirstFrameTimeoutMin": g.FirstFrameTimeoutMin, "PostLoginRxIdleMinutes": g.PostLoginRxIdleMinutes,
		"Timeout":         g.Timeout,
		"SupportCompress": g.SupportCompress, "SupportCas": g.SupportCas, "SupportCasLink": g.SupportCasLink,
		"SupportCommTermianl": g.SupportCommTermianl, "SupportReplyHeart": g.SupportReplyHeart, "SupportReplyReport": g.SupportReplyReport,
		"ConfFilePath": g.ConfFilePath,
	}
}

// RedactEffectiveConfig 对敏感字段脱敏（复制 map，不修改入参）。
func RedactEffectiveConfig(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	for k, v := range out {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "password") || strings.Contains(lk, "secret") || strings.Contains(lk, "token") || strings.Contains(lk, "credential") {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				out[k] = redactedPlaceholder
				continue
			}
		}
	}
	// 桥接地址等可能含内网拓扑，统一脱敏非空值
	if v, ok := out["BridgeHost698"].(string); ok && strings.TrimSpace(v) != "" {
		out["BridgeHost698"] = redactedPlaceholder
	}
	return out
}

const redactedPlaceholder = "（已脱敏）"
