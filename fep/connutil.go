package fep

import (
	"gfep/utils"
	"gfep/ziface"
	"gfep/znet"
	"log"
	"strconv"
	"time"
)

func asConn(c ziface.IConnection) *znet.Connection {
	co, _ := c.(*znet.Connection)
	return co
}

func touchRx(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastTouchRx(t)
		return
	}
	c.SetProperty("rtime", t)
}

func setRoutingAddr(c ziface.IConnection, addr string) {
	if co := asConn(c); co != nil {
		co.FastSetRoutingAddr(addr)
		return
	}
	c.SetProperty("addr", addr)
}

func setLtime(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastSetLtime(t)
		return
	}
	c.SetProperty("ltime", t)
}

// ensureSessionLtimeIfUnset 主站首帧起算「登录后」空闲超时：仅当 ltime 仍为零时写入。
func ensureSessionLtimeIfUnset(c ziface.IConnection, t time.Time) {
	v, err := c.GetProperty("ltime")
	if err != nil {
		return
	}
	lt, ok := v.(time.Time)
	if !ok || lt.IsZero() {
		setLtime(c, t)
	}
}

func setHtime(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastSetHtime(t)
		return
	}
	c.SetProperty("htime", t)
}

// setLastReportAt 698 终端上报：主站 MSA=0 且判定为上报帧时更新。
func setLastReportAt(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastSetLastReportAt(t)
		return
	}
	c.SetProperty("lastReportAt", t)
}

func setRoutingStatus(c ziface.IConnection, status int) {
	if co := asConn(c); co != nil {
		co.FastSetRoutingStatus(status)
		return
	}
	c.SetProperty("status", status)
}

// getConnStatus 读取业务状态；非 *znet.Connection 时回退 GetProperty。
// 与 ziface 其它实现并存时，status 解析失败返回 ok=false，调用方应 NeedStop。
func getConnStatus(c ziface.IConnection) (status int, ok bool) {
	if co := asConn(c); co != nil {
		s, _ := co.FastGetRouting()
		return s, true
	}
	v, err := c.GetProperty("status")
	if err != nil {
		return 0, false
	}
	s, tOK := v.(int)
	if !tOK {
		return 0, false
	}
	return s, true
}

// preAddrForTmnLogin 终端登录前已登记的地址（字符串）。
func preAddrForTmnLogin(c ziface.IConnection) string {
	if co := asConn(c); co != nil {
		_, a := co.FastGetRouting()
		return a
	}
	v, err := c.GetProperty("addr")
	if err != nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// msaString 将 MSA 格式化为十进制字符串（小整数路径由编译器优化，避免与 sync.Pool 共享底层字节导致数据竞态）。
func msaString(msa int) string {
	return strconv.Itoa(msa)
}

// 报文日志业务分类（日志行格式 [FROM->TO][CAT]；中文说明见各常量注释）。
const (
	logCatLink   = "LINK"       // login / heartbeat / logout / Online
	logCatFwd    = "FORWARD"    // master read, terminal response, passthrough
	logCatReport = "REPORT"     // terminal-initiated report
	logCatRptAck = "REPORT_ACK" // FEP report acknowledgment
)

// logPktLine 在 LogPacketHex 打开时打印一行：方向 + 分类 + 相关 conn + hex。
// from/to 建议使用 DCU、FEP、APP、BRG；connID 为与本端相关的连接号。
func logPktLine(lg *log.Logger, from, to, cat string, connID uint32, data []byte) {
	if lg == nil || !utils.GlobalObject.LogPacketHex {
		return
	}
	if len(data) == 0 {
		lg.Printf("[%s->%s][%s] conn=%d (empty)\n", from, to, cat, connID)
		return
	}
	if utils.GlobalObject.LogForwardEgressHex {
		lg.Printf("[%s->%s,%s,conn=%d] %X\n", from, to, cat, connID, data)
	} else {
		lg.Printf("[%s->%s][%s] %X\n", from, to, cat, data)
	}
}
