package bridge

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"gfep/zptl"
)

// Snapshot 供 Web 控制台展示的桥接链路快照（至 698 主站方向）。
type Snapshot struct {
	Host       string
	AddrHex    string
	ProtoLabel string
	Status     string // unconnected | tcp_connected | login_ok | exit_requested
	StatusZh   string
	TcpSince   time.Time
	LoginTime  time.Time
	HeartTime  time.Time
	LastRx     time.Time
	LastTx     time.Time
	RxBytes    uint64
	TxBytes    uint64
	RxPkts     uint64
	TxPkts     uint64
	HeartUnAck int32
}

func protoLabel(ptype uint32) string {
	if ptype&zptl.PTL_698_45 != 0 {
		return "698.45"
	}
	if ptype&zptl.PTL_1376_1 != 0 {
		return "376.1"
	}
	if ptype&zptl.PTL_NW != 0 {
		return "NW"
	}
	return fmt.Sprintf("0x%x", ptype)
}

func statusSnapshot(st connStatus) (en, zh string) {
	switch st {
	case unConnect:
		return "unconnected", "未连接"
	case tcpConnectOK:
		return "tcp_connected", "TCP 已连"
	case loginOK:
		return "login_ok", "已登录"
	case exitRequest:
		return "exit_requested", "退出中"
	default:
		return "unknown", "未知"
	}
}

func (c *Conn) writeToMaster(nc net.Conn, p []byte) error {
	if nc == nil || len(p) == 0 {
		return nil
	}
	n, err := nc.Write(p)
	if n > 0 {
		c.txBytes.Add(uint64(n))
		c.txPkts.Add(1)
		c.lastTxUnixNano.Store(time.Now().UnixNano())
	}
	return err
}

func (c *Conn) addRxRaw(n int) {
	if n <= 0 {
		return
	}
	c.rxBytes.Add(uint64(n))
	c.lastRxUnixNano.Store(time.Now().UnixNano())
}

func (c *Conn) incRxPkt() {
	c.rxPkts.Add(1)
}

func nanoOrZero(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// Snapshot 返回当前桥接状态（可并发调用；与 serve/recv 细粒度交错一致即可）。
func (c *Conn) Snapshot() Snapshot {
	st := connStatus(c.status.Load())
	en, zh := statusSnapshot(st)
	c.mu.Lock()
	host := c.host
	addr := append([]byte(nil), c.addr...)
	lt, ht := c.loginTime, c.heartTime
	tcpSince := c.tcpSince
	c.mu.Unlock()
	addrHex := fmt.Sprintf("%X", addr)
	lr := nanoOrZero(c.lastRxUnixNano.Load())
	lx := nanoOrZero(c.lastTxUnixNano.Load())
	return Snapshot{
		Host:       host,
		AddrHex:    addrHex,
		ProtoLabel: protoLabel(c.ptype),
		Status:     en,
		StatusZh:   zh,
		TcpSince:   tcpSince,
		LoginTime:  lt,
		HeartTime:  ht,
		LastRx:     lr,
		LastTx:     lx,
		RxBytes:    c.rxBytes.Load(),
		TxBytes:    c.txBytes.Load(),
		RxPkts:     c.rxPkts.Load(),
		TxPkts:     c.txPkts.Load(),
		HeartUnAck: atomic.LoadInt32(&c.heartUnAck),
	}
}

func resetBridgeTrafficCounters(c *Conn) {
	c.rxBytes.Store(0)
	c.txBytes.Store(0)
	c.rxPkts.Store(0)
	c.txPkts.Store(0)
	c.lastRxUnixNano.Store(0)
	c.lastTxUnixNano.Store(0)
}
